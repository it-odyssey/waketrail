package cmd

import (
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/it-odyssey/waketrail/internal/storage"
)

func composeEffect(resource, service, kind, summary string) storage.TimelineEvent {
	return storage.TimelineEvent{
		Source: "docker", ResourceType: "container", Resource: resource,
		EventType: kind, Summary: resource + " " + summary + " [compose:waketrail-smoke/" + service + "]",
	}
}

func TestComposeObservedTransitions(t *testing.T) {
	cases := []struct {
		name    string
		effects []storage.TimelineEvent
		want    []activityTransition
	}{
		{"up direct to running", []storage.TimelineEvent{
			composeEffect("web-1", "web", "created", "appeared: running"),
			composeEffect("api-1", "api", "created", "appeared: running"),
		}, []activityTransition{{"created", 0, 2}, {"running", 0, 2}, {"services", 0, 2}}},
		{"up intermediate startup and replicas", []storage.TimelineEvent{
			composeEffect("web-1", "web", "created", "appeared: created"),
			composeEffect("web-2", "web", "created", "appeared: running"),
			{Source: "docker", Resource: "web-1", EventType: "state_change", Summary: "web-1: created -> running [compose:waketrail-smoke/web]"},
		}, []activityTransition{{"created", 0, 2}, {"running", 0, 2}, {"services", 0, 1}}},
		{"down with observed running endpoints", []storage.TimelineEvent{
			{Source: "docker", Resource: "web-1", EventType: "stopped", Summary: "web-1: running -> exited (0) [compose:waketrail-smoke/web]"},
			{Source: "docker", Resource: "api-1", EventType: "stopped", Summary: "api-1: running -> exited (0) [compose:waketrail-smoke/api]"},
			composeEffect("web-1", "web", "removed", "disappeared"),
			composeEffect("api-1", "api", "removed", "disappeared"),
		}, []activityTransition{{"running", 2, 0}, {"removed", 0, 2}, {"services", 2, 0}}},
		{"down missing prior state", []storage.TimelineEvent{
			composeEffect("web-1", "web", "removed", "disappeared"),
			composeEffect("api-1", "api", "removed", "disappeared"),
		}, []activityTransition{{"removed", 0, 2}, {"services", 2, 0}}},
		{"health evidence", []storage.TimelineEvent{
			composeEffect("web-1", "web", "created", "appeared: running/starting"),
			{Source: "docker", Resource: "web-1", EventType: "state_change", Summary: "web-1: running/starting -> running/healthy [compose:waketrail-smoke/web]"},
		}, []activityTransition{{"created", 0, 1}, {"running", 0, 1}, {"healthy", 0, 1}, {"services", 0, 1}}},
		{"start existing stopped service", []storage.TimelineEvent{
			{Source: "docker", Resource: "web-1", EventType: "started", Summary: "web-1: exited (0) -> running [compose:waketrail-smoke/web]"},
		}, []activityTransition{{"running", 0, 1}}},
		{"restart preserves net state", []storage.TimelineEvent{
			{Source: "docker", Resource: "web-1", EventType: "stopped", Summary: "web-1: running -> exited (0) [compose:waketrail-smoke/web]"},
			{Source: "docker", Resource: "web-1", EventType: "started", Summary: "web-1: exited (0) -> running [compose:waketrail-smoke/web]"},
		}, nil},
		{"failure remains visible", []storage.TimelineEvent{
			{Source: "docker", Resource: "web-1", EventType: "failure", Summary: "web-1: running -> restarting [compose:waketrail-smoke/web]"},
		}, []activityTransition{{"running", 1, 0}, {"failures", 0, 1}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := summarizeComposeTransitions(tc.effects); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("transitions = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestComposeSharedCardAndVerboseEvidence(t *testing.T) {
	now := time.Date(2026, 10, 7, 19, 0, 0, 0, time.UTC)
	command := storage.CommandEvent{Command: "docker compose up -d", Cwd: "/tmp/different-directory", StartedAt: now, EndedAt: now.Add(time.Second)}
	timeline := []storage.TimelineEvent{
		composeEffect("web-1", "web", "created", "appeared: running"),
		composeEffect("api-1", "api", "created", "appeared: running"),
	}
	for i := range timeline {
		timeline[i].OccurredAt = now.Add(time.Duration(i+2) * time.Second)
	}
	original := append([]storage.TimelineEvent(nil), timeline...)
	events := buildDisplayEvents([]storage.CommandEvent{command}, timeline, false)
	if len(events) != 1 || events[0].Activity == nil {
		t.Fatalf("expected one activity, got %+v", events)
	}
	activity := *events[0].Activity
	fields := activityCardFields(activity)
	if fields[0] != (cardField{"Deployment", "waketrail-smoke"}) || fields[1].Label != "Effects" || fields[len(fields)-1] != (cardField{"Command", command.Command}) {
		t.Fatalf("wrong card grammar: %+v", fields)
	}
	wantEffects := "created   0 → 2\nrunning   0 → 2\nservices  0 → 2"
	if fields[1].Value != wantEffects {
		t.Fatalf("effects = %q, want %q", fields[1].Value, wantEffects)
	}
	renderer := lipgloss.NewRenderer(io.Discard)
	body := formatCardFields(renderer, fields)
	for _, line := range strings.Split(body, "\n") {
		if lipgloss.Width(line) > collectorCardContentWidth {
			t.Fatalf("card exceeds width: %q", line)
		}
	}
	var markdown strings.Builder
	writeMarkdownActivity(&markdown, activity)
	if !strings.Contains(markdown.String(), "### Docker Compose: UP") || !strings.Contains(markdown.String(), wantEffects) || !strings.Contains(markdown.String(), "**Deployment:** `waketrail-smoke`") {
		t.Fatalf("export lost shared effects: %s", markdown.String())
	}
	for _, forbidden := range []string{"Last seen", "Changes", "ready", "0 → 0", "[compose:"} {
		if strings.Contains(body, forbidden) || strings.Contains(markdown.String(), forbidden) {
			t.Fatalf("unexpected presentation %q", forbidden)
		}
	}
	verbose := buildDisplayEvents([]storage.CommandEvent{command}, timeline, true)
	if len(verbose) != 3 || !reflect.DeepEqual(timeline, original) || !reflect.DeepEqual(activity.Effects, original) {
		t.Fatal("grouping lost or modified raw evidence")
	}
	for _, event := range verbose {
		if event.Activity != nil {
			t.Fatal("verbose must retain per-container events")
		}
	}
	t.Log("Shared terminal fields:\n" + body)
}

func TestActivityDetailFallbackUsesSharedEffects(t *testing.T) {
	activity := displayActivity{Source: "docker", ResourceType: "container", Resource: "web-1", Effects: []storage.TimelineEvent{
		composeEffect("web-1", "web", "created", "appeared: created"),
		{Source: "docker", ResourceType: "container", Resource: "web-1", Summary: "web-1: created -> running [compose:waketrail-smoke/web]"},
	}}
	fields := activityCardFields(activity)
	if fields[1].Label != "Effects" || fields[1].Value != "Container: web-1\n  Appeared → created → running" {
		t.Fatalf("bad detail fallback: %+v", fields)
	}
}
