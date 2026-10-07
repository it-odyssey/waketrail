package cmd

import (
	"github.com/it-odyssey/waketrail/internal/storage"
	"strings"
	"testing"
	"time"
)

func TestRolloutSummaryUsesFinalObservedState(t *testing.T) {
	now := time.Now()
	activity := displayActivity{EventType: "apply", Effects: []storage.TimelineEvent{
		{ResourceType: "deployment", Resource: "demo/web", Summary: "appeared: desired 1 updated 0 ready 0 available 0", OccurredAt: now},
		{ResourceType: "deployment", Resource: "demo/web", Summary: "desired 1 updated 0 ready 0 available 0 → desired 1 updated 1 ready 1 available 1", OccurredAt: now.Add(time.Minute)},
		{ResourceType: "pod", Resource: "demo/web-abc", Summary: "appeared: Pending/ContainerCreating ready 0/1", OccurredAt: now},
		{ResourceType: "pod", Resource: "demo/web-abc", Summary: "Pending/ContainerCreating ready 0/1 → Running ready 1/1", OccurredAt: now.Add(time.Minute)},
	}}
	got := summarizeKubernetesRollout(activity)
	if got.Controllers != 1 || got.ControllersReady != 1 || got.Pods != 1 || got.PodsReady != 1 {
		t.Fatalf("unexpected readiness summary: %+v", got)
	}
}

func TestNormalStartupDoesNotCountAsIncident(t *testing.T) {
	cases := []storage.TimelineEvent{
		{Source: "kubernetes", ResourceType: "pod", EventType: "failure", Summary: "Pending/ContainerCreating ready 0/1 → Running ready 0/1"},
		{Source: "kubernetes", ResourceType: "pod", EventType: "recovery", Summary: "Running ready 0/1 → Running ready 1/1"},
		{Source: "kubernetes", ResourceType: "deployment", EventType: "recovery", Summary: "desired 1 updated 1 ready 0 available 0 → desired 1 updated 1 ready 1 available 1"},
	}
	for _, event := range cases {
		if got := normalizedTimelineEventType(event); got != "state_change" {
			t.Errorf("got %q for %+v", got, event)
		}
	}
}

func TestMiddleEllipsisKeepsSuffixAndFits(t *testing.T) {
	original := "Pod: online-boutique/productcatalogservice-8485d7d86d-lvmj2"
	got := ellipsizeMiddle(original, 42)
	if len([]rune(got)) > 42 || !strings.Contains(got, "…") || !strings.HasSuffix(got, "lvmj2") {
		t.Fatalf("bad ellipsis %q", got)
	}
}
