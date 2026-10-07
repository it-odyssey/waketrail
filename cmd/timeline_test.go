package cmd

import (
	"github.com/it-odyssey/waketrail/internal/storage"
	"testing"
	"time"
)

func TestLegacyDockerLifecycleAttribution(t *testing.T) {
	now := time.Now()
	command := storage.CommandEvent{Command: "docker run -d --name web nginx:alpine", StartedAt: now, EndedAt: now.Add(time.Second)}
	inspection := storage.CommandEvent{Command: "docker ps", StartedAt: now.Add(2 * time.Second), EndedAt: now.Add(3 * time.Second)}
	created := storage.TimelineEvent{Source: "docker", EventType: "state_change", ResourceType: "container", Resource: "web", Summary: "web appeared: running", OccurredAt: now.Add(4 * time.Second)}
	got := buildDisplayEvents([]storage.CommandEvent{command, inspection}, []storage.TimelineEvent{created}, false)
	if len(got) != 2 {
		t.Fatalf("events=%d want=2", len(got))
	}
	if got[1].CorrelatedCommand == nil || got[1].CorrelatedCommand.Command != command.Command {
		t.Fatal("legacy created event not attributed to docker run")
	}
	if kind := normalizedTimelineEventType(created); kind != "created" {
		t.Fatalf("legacy event kind=%q", kind)
	}
}

func TestDockerAbsentWatcherDoesNotCreateEvents(t *testing.T) {
	now := time.Now()
	command := storage.CommandEvent{Command: "docker run -d --name web nginx:alpine", StartedAt: now, EndedAt: now.Add(time.Second)}
	got := buildDisplayEvents([]storage.CommandEvent{command}, nil, false)
	if len(got) != 1 || got[0].Kind != "command" {
		t.Fatalf("expected standalone command when watcher has no observations: %+v", got)
	}
}

func TestKubernetesApplyGroupsNamespaceAndPreservesOtherEvents(t *testing.T) {
	now := time.Now()
	cmd := storage.CommandEvent{Command: "kubectl apply --namespace online-boutique --filename boutique.yaml", StartedAt: now, EndedAt: now.Add(time.Second)}
	deployment := storage.TimelineEvent{Source: "kubernetes", EventType: "state_change", ResourceType: "deployment", Resource: "online-boutique/frontend", Summary: "appeared: desired 1 ready 0", OccurredAt: now.Add(4 * time.Second)}
	other := storage.TimelineEvent{Source: "kubernetes", EventType: "state_change", ResourceType: "daemonset", Resource: "kube-system/unrelated", Summary: "appeared: desired 3", OccurredAt: now.Add(5 * time.Second)}
	ready := storage.TimelineEvent{Source: "kubernetes", EventType: "recovery", ResourceType: "deployment", Resource: "online-boutique/frontend", Summary: "ready 0 -> ready 1", OccurredAt: now.Add(55 * time.Second)}
	got := buildDisplayEvents([]storage.CommandEvent{cmd}, []storage.TimelineEvent{deployment, other, ready}, false)
	if len(got) != 2 {
		t.Fatalf("events=%d want 2", len(got))
	}
	if got[0].Activity == nil || got[0].Activity.EventType != "apply" || len(got[0].Activity.Effects) != 2 {
		t.Fatalf("grouped activity=%+v", got[0].Activity)
	}
	if got[1].TimelineEvent == nil || got[1].TimelineEvent.Resource != "kube-system/unrelated" {
		t.Fatal("unrelated event should remain visible")
	}
}

func TestControllerAppearanceSummarizesFinalReadiness(t *testing.T) {
	lines := formatControllerEffectLines("Appeared → desired 1 updated 0 ready 0 available 0 → desired 1 updated 1 ready 1 available 1")
	if len(lines) < 3 || lines[0] != "Appeared" {
		t.Fatalf("unexpected controller summary: %v", lines)
	}
}
