package cmd

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/it-odyssey/waketrail/internal/storage"
	"github.com/it-odyssey/waketrail/internal/ui"
)

const podsWide = `NAME READY STATUS RESTARTS AGE IP NODE NOMINATED NODE READINESS GATES
web-a 1/1 Running 0 1m 10.1.1.1 worker-1 <none> <none>
cache-b 0/1 ContainerCreating 0 1m <none> worker-2 <none> <none>
`

func TestPodObservationParsesWideSnapshot(t *testing.T) {
	cmd := storage.CommandEvent{Command: "kubectl get pods --namespace online-boutique --output wide"}
	obs, ok := parsePodObservation(cmd, storage.CommandOutput{Stdout: podsWide})
	if !ok || obs.Total != 2 || obs.Ready != 1 || obs.Running != 1 || obs.Restarts != 0 || obs.Namespace != "online-boutique" {
		t.Fatalf("parsed=%v observation=%+v", ok, obs)
	}
	if !strings.Contains(obs.Fields(cmd.Command)[1].Value, "1/2") {
		t.Fatalf("wrong readiness: %+v", obs)
	}
}
func TestPodObservationRejectsIncompleteAndStreams(t *testing.T) {
	for _, tc := range []struct {
		name, cmd, out string
		exit           int
		truncated      bool
	}{
		{"watch", "kubectl get pods -w", podsWide, 0, false},
		{"watch-long", "kubectl get pods --watch", podsWide, 0, false},
		{"json", "kubectl get pods -o json", podsWide, 0, false},
		{"failed", "kubectl get pods", podsWide, 1, false},
		{"truncated", "kubectl get pods", podsWide, 0, true},
		{"malformed", "kubectl get pods", "NAME READY STATUS\nweb-a 1/1 Running\n", 0, false},
		{"no-header", "kubectl get pods --no-headers", podsWide, 0, false},
		{"all-namespaces", "kubectl get pods -A", podsWide, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := parsePodObservation(storage.CommandEvent{Command: tc.cmd, ExitCode: tc.exit}, storage.CommandOutput{Stdout: tc.out, StdoutTruncated: tc.truncated})
			if ok {
				t.Fatalf("unsafe STATUS for %s", tc.name)
			}
		})
	}
}
func TestPodObservationDoesNotMistakeUnreadyForFailure(t *testing.T) {
	obs, ok := parsePodObservation(storage.CommandEvent{Command: "kubectl get pods"}, storage.CommandOutput{Stdout: podsWide})
	if !ok || obs.Ready == obs.Total {
		t.Fatalf("unexpected result: %+v", obs)
	}
	if strings.Contains(strings.ToLower(obs.Fields("kubectl get pods")[4].Value), "healthy") {
		t.Fatal("incomplete readiness described as healthy")
	}
}
func TestKubernetesHistoricalIncidentClassification(t *testing.T) {
	cases := []struct {
		event storage.TimelineEvent
		want  string
	}{
		{storage.TimelineEvent{Source: "kubernetes", ResourceType: "pod", EventType: "failure", Summary: "Pending/ContainerCreating ready 0/1 → Running ready 0/1"}, "state_change"},
		{storage.TimelineEvent{Source: "kubernetes", ResourceType: "pod", EventType: "recovery", Summary: "Running ready 0/1 → Running ready 1/1"}, "state_change"},
		{storage.TimelineEvent{Source: "kubernetes", ResourceType: "pod", EventType: "failure", Summary: "Running ready 1/1 → Running/CrashLoopBackOff ready 0/1"}, "failure"},
		{storage.TimelineEvent{Source: "kubernetes", ResourceType: "pod", EventType: "recovery", Summary: "Running/CrashLoopBackOff ready 0/1 → Running ready 1/1"}, "recovery"},
		{storage.TimelineEvent{Source: "kubernetes", ResourceType: "deployment", EventType: "recovery", Summary: "desired 1 updated 1 ready 0 available 0 → desired 1 updated 1 ready 1 available 1"}, "state_change"},
	}
	for _, tc := range cases {
		if got := normalizedTimelineEventType(tc.event); got != tc.want {
			t.Errorf("summary=%s got=%s want=%s", tc.event.Summary, got, tc.want)
		}
	}
}

func TestCardFieldsFitFixedWidth(t *testing.T) {
	renderer := ui.Renderer()
	long := "online-boutique/productcatalogservice-8485d7d86d-lvmj2"
	fields := []cardField{{Label: "Deployment", Value: long}, {Label: "Command", Value: "kubectl apply --namespace online-boutique --filename https://raw.githubusercontent.com/GoogleCloudPlatform/microservices-demo/v0/release/kubernetes-manifests.yaml"}}
	body := formatCardFields(renderer, fields)
	for _, line := range strings.Split(body, "\n") {
		if got := lipgloss.Width(line); got > collectorCardContentWidth {
			t.Fatalf("card content overflow: got %d, max %d: %q", got, collectorCardContentWidth, line)
		}
	}
}
