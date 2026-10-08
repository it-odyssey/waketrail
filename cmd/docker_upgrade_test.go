package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/it-odyssey/waketrail/internal/storage"
)

func TestDockerObservationAllAndFiltered(t *testing.T) {
	table := "CONTAINER ID   IMAGE          COMMAND                  CREATED         STATUS                       PORTS     NAMES\n" +
		"aaa            nginx:alpine   \"nginx -g daemon off;\"   2 minutes ago   Up 2 minutes (healthy)                web\n" +
		"bbb            redis          \"docker-entrypoint.s\"    3 minutes ago   Exited (0) 10 seconds ago              cache\n"
	command := storage.CommandEvent{Command: "docker ps -a"}
	obs, ok := parseDockerObservation(command, storage.CommandOutput{Stdout: table})
	if !ok || !obs.All || obs.Listed != 2 || obs.Running != 1 || obs.Stopped != 1 || obs.Healthy != 1 {
		t.Fatalf("unexpected observation: %+v, ok=%v", obs, ok)
	}
	command.Command = "docker ps"
	obs, ok = parseDockerObservation(command, storage.CommandOutput{Stdout: table})
	if !ok || obs.All {
		t.Fatalf("filtered ps incorrectly treated as complete: %+v", obs)
	}
}
func TestDockerObservationRejectsIncompleteAndCustom(t *testing.T) {
	table := "CONTAINER ID   IMAGE   STATUS   NAMES\naaa            nginx   Up 1 minute   web\n"
	for _, tc := range []struct {
		command string
		output  storage.CommandOutput
		exit    int
	}{
		{"docker ps --format '{{.Names}}'", storage.CommandOutput{Stdout: table}, 0},
		{"docker ps -a", storage.CommandOutput{Stdout: table, StdoutTruncated: true}, 0},
		{"docker ps -a", storage.CommandOutput{Stdout: table}, 1},
		{"docker ps -a", storage.CommandOutput{Stdout: "garbage"}, 0},
	} {
		if _, ok := parseDockerObservation(storage.CommandEvent{Command: tc.command, ExitCode: tc.exit}, tc.output); ok {
			t.Fatalf("accepted unsafe snapshot for %q", tc.command)
		}
	}
}
func TestComposeMetadataAndGrouping(t *testing.T) {
	now := time.Now()
	command := storage.CommandEvent{Command: "docker compose -p monitoring up -d", Cwd: "/tmp/irrelevant", StartedAt: now, EndedAt: now.Add(time.Second)}
	events := []displayEvent{{Kind: "command", OccurredAt: now, CommandEvent: &command}}
	for i, name := range []string{"web", "api", "database"} {
		e := storage.TimelineEvent{Source: "docker", EventType: "created", ResourceType: "container", Resource: name, Summary: name + " appeared: running [compose:monitoring/" + name + "]", OccurredAt: now.Add(time.Duration(2+i) * time.Second)}
		events = append(events, displayEvent{Kind: "timeline", OccurredAt: e.OccurredAt, TimelineEvent: &e})
	}
	unrelated := storage.TimelineEvent{Source: "docker", EventType: "created", Resource: "outsider", Summary: "outsider appeared: running [compose:other/web]", OccurredAt: now.Add(6 * time.Second)}
	events = append(events, displayEvent{Kind: "timeline", OccurredAt: unrelated.OccurredAt, TimelineEvent: &unrelated})
	got := groupDockerActivities(events)
	if len(got) != 2 || got[0].Kind != "activity" || got[0].Activity.Source != "docker compose" || len(got[0].Activity.Effects) != 3 || got[1].TimelineEvent.Resource != "outsider" {
		t.Fatalf("grouping failed: %+v", got)
	}
	if !strings.Contains(activityCardFields(*got[0].Activity)[1].Value, "3") {
		t.Fatal("missing count")
	}
}
func TestDockerCreationGroupPreservesEvidence(t *testing.T) {
	now := time.Now()
	command := storage.CommandEvent{Command: "docker run -d --name sample nginx:alpine", StartedAt: now, EndedAt: now.Add(time.Second)}
	created := storage.TimelineEvent{Source: "docker", EventType: "created", Resource: "sample", Summary: "sample appeared: created", OccurredAt: now.Add(2 * time.Second)}
	running := storage.TimelineEvent{Source: "docker", EventType: "state_change", Resource: "sample", Summary: "sample: created -> running", OccurredAt: now.Add(4 * time.Second)}
	events := []displayEvent{{OccurredAt: created.OccurredAt, Kind: "timeline", TimelineEvent: &created, CorrelatedCommand: &command}, {OccurredAt: running.OccurredAt, Kind: "timeline", TimelineEvent: &running}}
	got := groupDockerActivities(events)
	if len(got) != 1 || got[0].Kind != "activity" || len(got[0].Activity.Effects) != 2 {
		t.Fatalf("expected one activity preserving 2 effects, got %+v", got)
	}
}
func TestComposeRequiresMatchingProject(t *testing.T) {
	now := time.Now()
	cmd := storage.CommandEvent{Command: "docker compose -p monitoring down", StartedAt: now, EndedAt: now.Add(time.Second)}
	e := storage.TimelineEvent{Source: "docker", EventType: "removed", Resource: "web", Summary: "web disappeared [compose:unrelated/web]", OccurredAt: now.Add(2 * time.Second)}
	got := groupDockerActivities([]displayEvent{{Kind: "command", OccurredAt: now, CommandEvent: &cmd}, {Kind: "timeline", OccurredAt: e.OccurredAt, TimelineEvent: &e}})
	if len(got) != 2 || got[0].Kind != "command" {
		t.Fatalf("misattributed different project: %+v", got)
	}
}

func TestDockerComposePSStatus(t *testing.T) {
	table := "NAME                IMAGE             COMMAND               SERVICE    CREATED         STATUS          PORTS\n" +
		"monitoring-web-1    nginx:alpine      \"nginx -g daemon off\"  web        2 minutes ago   Up 2 minutes    0.0.0.0:80->80/tcp\n" +
		"monitoring-db-1     postgres:16        \"docker-entrypoint\"   database   2 minutes ago   Exited (0) 1m\n"
	command := storage.CommandEvent{Command: "docker compose -p monitoring ps -a"}
	obs, ok := parseDockerObservation(command, storage.CommandOutput{Stdout: table})
	if !ok || obs.Scope != "Compose project monitoring" || !obs.All || obs.Listed != 2 || obs.Running != 1 {
		t.Fatalf("compose observation: %+v, ok=%v", obs, ok)
	}
}

func TestComposeInfersProjectFromObservedLabelsWhenComposeNameDiffersFromDirectory(t *testing.T) {
	now := time.Now()
	command := storage.CommandEvent{
		Command:   "docker compose up -d",
		Cwd:       "/tmp/waketrail-compose-smoke",
		StartedAt: now,
		EndedAt:   now.Add(time.Second),
	}
	events := []displayEvent{{Kind: "command", OccurredAt: now, CommandEvent: &command}}
	for i, service := range []string{"api", "web"} {
		e := storage.TimelineEvent{
			Source:       "docker",
			EventType:    "created",
			ResourceType: "container",
			Resource:     "waketrail-smoke-" + service + "-1",
			Summary:      "appeared: running [compose:waketrail-smoke/" + service + "]",
			OccurredAt:   now.Add(time.Duration(i+2) * time.Second),
		}
		events = append(events, displayEvent{Kind: "timeline", OccurredAt: e.OccurredAt, TimelineEvent: &e})
	}

	got := groupDockerActivities(events)
	if len(got) != 1 || got[0].Kind != "activity" {
		t.Fatalf("expected grouped compose activity, got %+v", got)
	}
	if got[0].Activity.Resource != "waketrail-smoke" || len(got[0].Activity.Effects) != 2 {
		t.Fatalf("wrong inferred project/activity: %+v", got[0].Activity)
	}
}

func TestComposeWithoutExplicitProjectRejectsAmbiguousProjects(t *testing.T) {
	now := time.Now()
	command := storage.CommandEvent{Command: "docker compose up -d", Cwd: "/tmp/no-match", StartedAt: now, EndedAt: now.Add(time.Second)}
	events := []displayEvent{{Kind: "command", OccurredAt: now, CommandEvent: &command}}
	for i, project := range []string{"alpha", "beta"} {
		e := storage.TimelineEvent{Source: "docker", EventType: "created", Resource: project + "-web-1", Summary: "appeared: running [compose:" + project + "/web]", OccurredAt: now.Add(time.Duration(i+2) * time.Second)}
		events = append(events, displayEvent{Kind: "timeline", OccurredAt: e.OccurredAt, TimelineEvent: &e})
	}
	got := groupDockerActivities(events)
	if len(got) != 3 || got[0].Kind != "command" {
		t.Fatalf("ambiguous projects should stay ungrouped: %+v", got)
	}
}

func TestDockerObservationAllowsFilteredAllListing(t *testing.T) {
	table := "CONTAINER ID   IMAGE          COMMAND                  CREATED          STATUS          PORTS     NAMES\n" +
		"aaa            nginx:alpine   \"/docker-entrypoint…\"   42 seconds ago   Up 41 seconds   80/tcp    waketrail-smoke-api-1\n" +
		"bbb            nginx:alpine   \"/docker-entrypoint…\"   42 seconds ago   Up 41 seconds   80/tcp    waketrail-smoke-web-1\n"
	command := storage.CommandEvent{Command: "docker ps -a --filter label=com.docker.compose.project=waketrail-smoke"}
	obs, ok := parseDockerObservation(command, storage.CommandOutput{Stdout: table})
	if !ok {
		t.Fatal("filtered docker ps should produce a semantic observation")
	}
	if !obs.All || !obs.Filtered || obs.Listed != 2 || obs.Running != 2 || obs.Stopped != 0 {
		t.Fatalf("unexpected filtered observation: %+v", obs)
	}
	if obs.Scope != "Filtered containers" {
		t.Fatalf("Scope = %q", obs.Scope)
	}
}
