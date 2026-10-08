package cmd

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/it-odyssey/waketrail/internal/storage"
)

const activityWindow = 30 * time.Second

type displayActivity struct {
	OccurredAt   time.Time
	Source       string
	EventType    string
	ResourceType string
	Resource     string
	Command      *storage.CommandEvent
	Effects      []storage.TimelineEvent
	// Transitions are semantic aggregate effects, independent of card layout.
	// Effects still holds the original per-resource evidence.
	Transitions        []activityTransition
	EffectScope        string
	HasObservedFailure bool
}

type activityTransition struct {
	Name   string
	Before int
	After  int
}

type kubernetesCommandTarget struct {
	Operation    string
	ResourceType string
	Resource     string
	Namespace    string
}

func groupKubernetesActivities(
	events []displayEvent,
) []displayEvent {
	result := make(
		[]displayEvent,
		0,
		len(events),
	)

	consumed := make(map[int]bool)
	for i := 0; i < len(events); i++ {
		if consumed[i] {
			continue
		}
		event := events[i]

		if event.Kind != "command" ||
			event.CommandEvent == nil {
			result = append(result, event)
			continue
		}

		target, ok := kubernetesTargetFromCommand(
			event.CommandEvent.Command,
		)
		if !ok {
			result = append(result, event)
			continue
		}

		// Failed commands cannot have a successful apply activity.
		if event.CommandEvent.ExitCode != 0 {
			result = append(result, event)
			continue
		}
		var effects []storage.TimelineEvent
		window := activityWindow
		if target.Operation == "apply" {
			window = 2 * time.Minute
		}

		for j := i + 1; j < len(events); j++ {
			candidate := events[j]

			// Other commands do not invalidate observations already underway.
			// A later mutating Kubernetes command, however, ends attribution.
			if candidate.Kind == "command" && candidate.CommandEvent != nil {
				if nextTarget, ok := kubernetesTargetFromCommand(candidate.CommandEvent.Command); ok && nextTarget.Operation != "" {
					break
				}
				continue
			}

			if candidate.OccurredAt.Sub(
				event.CommandEvent.EndedAt,
			) > window {
				break
			}

			if candidate.Kind != "timeline" ||
				candidate.TimelineEvent == nil {
				continue
			}

			if !kubernetesEventMatchesTarget(
				*candidate.TimelineEvent,
				target,
			) {
				continue
			}

			effects = append(
				effects,
				*candidate.TimelineEvent,
			)

			consumed[j] = true
		}

		if len(effects) == 0 {
			result = append(result, event)
			continue
		}

		resource := target.Resource
		resourceType := target.ResourceType
		if target.Operation == "apply" {
			resourceType, resource = "namespace", target.Namespace
		}

		for _, effect := range effects {
			if target.Operation != "apply" && effect.ResourceType ==
				target.ResourceType {
				resource = effect.Resource
				break
			}
		}

		activity := displayActivity{
			OccurredAt:   event.CommandEvent.StartedAt,
			Source:       "kubernetes",
			EventType:    activityEventType(effects),
			ResourceType: resourceType,
			Resource:     resource,
			Command:      event.CommandEvent,
			Effects:      effects,
		}

		if target.Operation == "apply" {
			activity.EventType = "apply"
		}
		result = append(
			result,
			displayEvent{
				OccurredAt: event.CommandEvent.StartedAt,
				Kind:       "activity",
				Activity:   &activity,
			},
		)

	}

	return result
}

func kubernetesTargetFromCommand(
	command string,
) (kubernetesCommandTarget, bool) {
	fields := strings.Fields(command)
	fields = stripCommandPrefixes(fields)

	if len(fields) < 3 ||
		filepath.Base(fields[0]) != "kubectl" {
		return kubernetesCommandTarget{}, false
	}

	namespace := namespaceFromKubectlFields(fields)

	switch fields[1] {
	case "apply":
		// Only namespace-scoped Kubernetes observations can be safely tied
		// to an apply without parsing the referenced manifest or owner UIDs.
		if namespace == "" {
			return kubernetesCommandTarget{}, false
		}
		return kubernetesCommandTarget{Operation: "apply", Namespace: namespace, ResourceType: "namespace", Resource: namespace}, true
	case "scale":
		return parseKubernetesScaleTarget(
			fields,
			namespace,
		)

	default:
		return kubernetesCommandTarget{}, false
	}
}

func parseKubernetesScaleTarget(
	fields []string,
	namespace string,
) (kubernetesCommandTarget, bool) {
	if len(fields) < 4 {
		return kubernetesCommandTarget{}, false
	}

	resourceType := normalizeKubernetesResourceType(
		fields[2],
	)

	resource := fields[3]

	if strings.Contains(fields[2], "/") {
		parts := strings.SplitN(
			fields[2],
			"/",
			2,
		)

		resourceType =
			normalizeKubernetesResourceType(
				parts[0],
			)

		resource = parts[1]
	}

	if resourceType == "" ||
		resource == "" {
		return kubernetesCommandTarget{}, false
	}

	return kubernetesCommandTarget{
		Operation:    "scale",
		ResourceType: resourceType,
		Resource:     resource,
		Namespace:    namespace,
	}, true
}

func normalizeKubernetesResourceType(
	value string,
) string {
	switch value {
	case "deployment",
		"deploy",
		"deployments":
		return "deployment"

	case "statefulset",
		"statefulsets",
		"sts":
		return "statefulset"

	case "daemonset",
		"daemonsets",
		"ds":
		return "daemonset"

	case "pod",
		"pods",
		"po":
		return "pod"

	default:
		return ""
	}
}

func namespaceFromKubectlFields(
	fields []string,
) string {
	for i := 0; i < len(fields); i++ {
		switch fields[i] {
		case "-n", "--namespace":
			if i+1 < len(fields) {
				return fields[i+1]
			}
		}

		if strings.HasPrefix(
			fields[i],
			"--namespace=",
		) {
			return strings.TrimPrefix(
				fields[i],
				"--namespace=",
			)
		}
	}

	return ""
}

func kubernetesEventMatchesTarget(
	event storage.TimelineEvent,
	target kubernetesCommandTarget,
) bool {
	if event.Source != "kubernetes" {
		return false
	}
	if target.Operation == "apply" && event.ResourceType != "pod" && event.ResourceType != "deployment" && event.ResourceType != "statefulset" && event.ResourceType != "daemonset" {
		return false
	}

	resourceNamespace, resourceName :=
		splitKubernetesResource(event.Resource)

	if target.Namespace != "" &&
		resourceNamespace != target.Namespace {
		return false
	}

	if target.Operation == "apply" {
		return true
	}
	if event.ResourceType ==
		target.ResourceType {
		return resourceName == target.Resource
	}

	// A Deployment owns ReplicaSets which generate Pods whose names
	// begin with the Deployment name. This lets the presentation
	// correlate rollout effects without changing the forensic events.
	if target.ResourceType == "deployment" &&
		event.ResourceType == "pod" {
		return strings.HasPrefix(
			resourceName,
			target.Resource+"-",
		)
	}

	return false
}

func splitKubernetesResource(
	resource string,
) (string, string) {
	parts := strings.SplitN(
		resource,
		"/",
		2,
	)

	if len(parts) == 1 {
		return "", parts[0]
	}

	return parts[0], parts[1]
}

func activityEventType(
	effects []storage.TimelineEvent,
) string {
	hasFailure := false

	for _, effect := range effects {
		switch normalizedTimelineEventType(effect) {
		case "recovery":
			// If the activity experienced a transient failure but
			// recovered during the same operation, present the final
			// successful outcome rather than a top-level incident.
			return "recovery"

		case "failure":
			hasFailure = true
		}
	}

	if hasFailure {
		return "failure"
	}

	return "state_change"
}

type displayEffect struct {
	ResourceType string
	Resource     string
	Summary      string
}

func summarizeActivityEffects(
	effects []storage.TimelineEvent,
) []displayEffect {
	var result []displayEffect

	indexByResource := make(map[string]int)

	for _, effect := range effects {
		key := effect.ResourceType +
			"\x00" +
			effect.Resource

		summary := effect.Summary
		if effect.Source == "docker" {
			summary, _, _ = dockerEffectSummary(timelineDisplaySummary(effect))
			summary = strings.TrimPrefix(summary, effect.Resource+" ")
		}
		summary = normalizeEffectSummary(summary)

		index, exists := indexByResource[key]
		if !exists {
			indexByResource[key] = len(result)

			result = append(
				result,
				displayEffect{
					ResourceType: effect.ResourceType,
					Resource:     effect.Resource,
					Summary:      summary,
				},
			)

			continue
		}

		result[index].Summary = mergeEffectSummaries(
			result[index].Summary,
			summary,
		)
	}

	return result
}

func normalizeEffectSummary(
	summary string,
) string {
	switch {
	case strings.HasPrefix(
		summary,
		"appeared: ",
	):
		return "Appeared → " +
			strings.TrimPrefix(
				summary,
				"appeared: ",
			)

	case summary == "disappeared":
		return "Disappeared"

	default:
		return formatTransitionArrow(summary)
	}
}

func mergeEffectSummaries(
	current string,
	next string,
) string {
	parts := strings.SplitN(
		next,
		" → ",
		2,
	)

	if len(parts) == 2 &&
		strings.HasSuffix(
			current,
			parts[0],
		) {
		return current +
			" → " +
			parts[1]
	}

	if next == "Disappeared" {
		return current + " → Disappeared"
	}

	return current + "; " + next
}

func displayResourceType(
	resourceType string,
) string {
	switch resourceType {
	case "pod":
		return "Pod"

	case "deployment":
		return "Deployment"

	case "statefulset":
		return "StatefulSet"

	case "daemonset":
		return "DaemonSet"

	case "node":
		return "Node"

	case "container":
		return "Container"

	case "service":
		return "Service"

	case "compose":
		return "Compose"

	default:
		return titleSource(resourceType)
	}
}

func formatActivityEffectLines(
	effect displayEffect,
) []string {
	switch effect.ResourceType {
	case "deployment",
		"statefulset",
		"daemonset":
		return formatControllerEffectLines(
			effect.Summary,
		)

	case "pod":
		return formatProgressionLines(
			effect.Summary,
		)

	case "node":
		return formatProgressionLines(
			effect.Summary,
		)

	default:
		return []string{effect.Summary}
	}
}

func formatControllerEffectLines(
	summary string,
) []string {
	states := strings.Split(
		summary,
		" → ",
	)

	if len(states) < 2 {
		return []string{summary}
	}

	// The normal report cares about the net result of the activity.
	// Intermediate controller states remain available in verbose/raw
	// output, but here we compare the first observed state to the last.
	appeared := states[0] == "Appeared"
	if appeared {
		states = states[1:]
	}
	if len(states) == 0 {
		return []string{"Appeared"}
	}
	before := parseStateFields(
		states[0],
	)

	after := parseStateFields(
		states[len(states)-1],
	)

	order := []string{
		"desired",
		"current",
		"updated",
		"ready",
		"available",
		"misscheduled",
	}

	var lines []string
	if appeared {
		lines = append(lines, "Appeared")
	}

	for _, field := range order {
		beforeValue, beforeExists :=
			before[field]

		afterValue, afterExists :=
			after[field]

		if !beforeExists || !afterExists {
			continue
		}

		if beforeValue == afterValue {
			continue
		}

		lines = append(
			lines,
			fmt.Sprintf(
				"%-12s %s → %s",
				field,
				beforeValue,
				afterValue,
			),
		)
	}

	if len(lines) == 0 {
		return []string{summary}
	}

	return lines
}

func parseStateFields(
	value string,
) map[string]string {
	fields := strings.Fields(value)

	result := make(map[string]string)

	for i := 0; i+1 < len(fields); i += 2 {
		result[fields[i]] = fields[i+1]
	}

	return result
}

func formatProgressionLines(
	summary string,
) []string {
	states := strings.Split(
		summary,
		" → ",
	)

	if len(states) <= 1 {
		return []string{summary}
	}

	lines := []string{states[0]}

	for _, state := range states[1:] {
		lines = append(
			lines,
			"→ "+state,
		)
	}

	return lines
}

// kubernetesRolloutSummary summarizes only resources actually observed by
// the collector. These counts do not assert cluster-wide completeness.
type kubernetesRolloutSummary struct {
	Controllers, ControllersReady int
	Pods, PodsReady               int
	Incidents                     int
	First, Last                   time.Time
}

func summarizeKubernetesRollout(activity displayActivity) kubernetesRolloutSummary {
	var result kubernetesRolloutSummary
	for _, event := range activity.Effects {
		if normalizedTimelineEventType(event) == "failure" {
			result.Incidents++
		}
		if result.First.IsZero() || event.OccurredAt.Before(result.First) {
			result.First = event.OccurredAt
		}
		if result.Last.IsZero() || event.OccurredAt.After(result.Last) {
			result.Last = event.OccurredAt
		}
	}
	for _, effect := range summarizeActivityEffects(activity.Effects) {
		states := strings.Split(effect.Summary, " → ")
		last := states[len(states)-1]
		switch effect.ResourceType {
		case "deployment", "statefulset", "daemonset":
			result.Controllers++
			fields := parseStateFields(last)
			desired, desiredOK := fields["desired"]
			ready, readyOK := fields["ready"]
			available, availableOK := fields["available"]
			// A controller is counted ready only when the final observed
			// ready and available replicas match its desired count.
			if desiredOK && readyOK && availableOK && desired == ready && desired == available {
				result.ControllersReady++
			}
		case "pod":
			result.Pods++
			// Running alone does not imply a ready Pod.
			parts := strings.Fields(last)
			if strings.HasPrefix(last, "Running ready ") && len(parts) >= 3 {
				counts := strings.Split(parts[2], "/")
				if len(counts) == 2 && counts[0] == counts[1] && counts[1] != "0" {
					result.PodsReady++
				}
			}
		}
	}
	return result
}
