package cmd

import (
	"testing"
	"time"

	"github.com/it-odyssey/waketrail/internal/storage"
)

func TestGroupKubernetesScaleActivity(
	t *testing.T,
) {
	started := time.Now()

	command := storage.CommandEvent{
		Command:   "kubectl scale deployment hello --replicas=1",
		StartedAt: started,
		EndedAt: started.Add(
			100 * time.Millisecond,
		),
	}

	events := []displayEvent{
		{
			OccurredAt:   command.StartedAt,
			Kind:         "command",
			CommandEvent: &command,
		},
		{
			OccurredAt: started.Add(time.Second),
			Kind:       "timeline",
			TimelineEvent: &storage.TimelineEvent{
				EventType:    "failure",
				Source:       "kubernetes",
				ResourceType: "deployment",
				Resource:     "default/hello",
				Summary:      "desired 0 -> desired 1",
			},
		},
		{
			OccurredAt: started.Add(
				2 * time.Second,
			),
			Kind: "timeline",
			TimelineEvent: &storage.TimelineEvent{
				EventType:    "state_change",
				Source:       "kubernetes",
				ResourceType: "pod",
				Resource:     "default/hello-abc123",
				Summary:      "appeared: Pending/ContainerCreating",
			},
		},
		{
			OccurredAt: started.Add(
				10 * time.Second,
			),
			Kind: "timeline",
			TimelineEvent: &storage.TimelineEvent{
				EventType:    "recovery",
				Source:       "kubernetes",
				ResourceType: "deployment",
				Resource:     "default/hello",
				Summary:      "available 0 -> available 1",
			},
		},
		{
			OccurredAt: started.Add(
				11 * time.Second,
			),
			Kind: "timeline",
			TimelineEvent: &storage.TimelineEvent{
				EventType:    "state_change",
				Source:       "kubernetes",
				ResourceType: "pod",
				Resource:     "default/hello-abc123",
				Summary:      "Pending/ContainerCreating -> Running ready 1/1",
			},
		},
	}

	grouped := groupKubernetesActivities(events)

	if len(grouped) != 1 {
		t.Fatalf(
			"len(grouped) = %d, want 1",
			len(grouped),
		)
	}

	activity := grouped[0].Activity

	if activity == nil {
		t.Fatal("expected grouped activity")
	}

	if activity.EventType != "recovery" {
		t.Fatalf(
			"EventType = %q, want recovery",
			activity.EventType,
		)
	}

	if activity.ResourceType != "deployment" {
		t.Fatalf(
			"ResourceType = %q, want deployment",
			activity.ResourceType,
		)
	}

	if activity.Resource != "default/hello" {
		t.Fatalf(
			"Resource = %q, want default/hello",
			activity.Resource,
		)
	}

	if len(activity.Effects) != 4 {
		t.Fatalf(
			"len(Effects) = %d, want 4",
			len(activity.Effects),
		)
	}
}
