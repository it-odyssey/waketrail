package kubernetes

import (
	"fmt"
	"sort"
)

const (
	EventStateChange = "state_change"
	EventFailure     = "failure"
	EventRecovery    = "recovery"
)

type Transition struct {
	ResourceType string
	Resource     string
	EventType    string
	Summary      string
}

func Compare(
	previous Snapshot,
	current Snapshot,
) []Transition {
	var transitions []Transition

	transitions = append(
		transitions,
		comparePods(
			previous.Pods,
			current.Pods,
		)...,
	)

	transitions = append(
		transitions,
		compareDeployments(
			previous.Deployments,
			current.Deployments,
		)...,
	)

	transitions = append(
		transitions,
		compareStatefulSets(
			previous.StatefulSets,
			current.StatefulSets,
		)...,
	)

	transitions = append(
		transitions,
		compareDaemonSets(
			previous.DaemonSets,
			current.DaemonSets,
		)...,
	)

	transitions = append(
		transitions,
		compareNodes(
			previous.Nodes,
			current.Nodes,
		)...,
	)

	sort.Slice(
		transitions,
		func(i, j int) bool {
			if transitions[i].ResourceType !=
				transitions[j].ResourceType {
				return transitions[i].ResourceType <
					transitions[j].ResourceType
			}

			return transitions[i].Resource <
				transitions[j].Resource
		},
	)

	return transitions
}

func comparePods(
	previous []PodState,
	current []PodState,
) []Transition {
	previousByName := make(
		map[string]PodState,
		len(previous),
	)

	currentByName := make(
		map[string]PodState,
		len(current),
	)

	for _, pod := range previous {
		previousByName[namespacedName(
			pod.Namespace,
			pod.Name,
		)] = pod
	}

	for _, pod := range current {
		currentByName[namespacedName(
			pod.Namespace,
			pod.Name,
		)] = pod
	}

	var transitions []Transition

	for name, currentState := range currentByName {
		previousState, existed :=
			previousByName[name]

		if !existed {
			transitions = append(
				transitions,
				Transition{
					ResourceType: "pod",
					Resource:     name,
					EventType:    EventStateChange,
					Summary: "appeared: " +
						describePod(currentState),
				},
			)

			continue
		}

		if podStatesEqual(
			previousState,
			currentState,
		) {
			continue
		}

		transitions = append(
			transitions,
			Transition{
				ResourceType: "pod",
				Resource:     name,
				EventType: classifyHealthTransition(
					podFailed(previousState),
					podFailed(currentState),
				),
				Summary: fmt.Sprintf(
					"%s -> %s",
					describePod(previousState),
					describePod(currentState),
				),
			},
		)
	}

	for name := range previousByName {
		if _, exists := currentByName[name]; exists {
			continue
		}

		transitions = append(
			transitions,
			Transition{
				ResourceType: "pod",
				Resource:     name,
				EventType:    EventStateChange,
				Summary:      "disappeared",
			},
		)
	}

	return transitions
}

func compareDeployments(
	previous []DeploymentState,
	current []DeploymentState,
) []Transition {
	previousByName := make(
		map[string]DeploymentState,
		len(previous),
	)

	currentByName := make(
		map[string]DeploymentState,
		len(current),
	)

	for _, deployment := range previous {
		previousByName[namespacedName(
			deployment.Namespace,
			deployment.Name,
		)] = deployment
	}

	for _, deployment := range current {
		currentByName[namespacedName(
			deployment.Namespace,
			deployment.Name,
		)] = deployment
	}

	var transitions []Transition

	for name, currentState := range currentByName {
		previousState, existed :=
			previousByName[name]

		if !existed {
			transitions = append(
				transitions,
				Transition{
					ResourceType: "deployment",
					Resource:     name,
					EventType:    EventStateChange,
					Summary: "appeared: " +
						describeDeployment(currentState),
				},
			)

			continue
		}

		if previousState == currentState {
			continue
		}

		transitions = append(
			transitions,
			Transition{
				ResourceType: "deployment",
				Resource:     name,
				EventType:    EventStateChange,
				Summary: fmt.Sprintf(
					"%s -> %s",
					describeDeployment(previousState),
					describeDeployment(currentState),
				),
			},
		)
	}

	for name := range previousByName {
		if _, exists := currentByName[name]; exists {
			continue
		}

		transitions = append(
			transitions,
			Transition{
				ResourceType: "deployment",
				Resource:     name,
				EventType:    EventStateChange,
				Summary:      "disappeared",
			},
		)
	}

	return transitions
}

func compareStatefulSets(
	previous []StatefulSetState,
	current []StatefulSetState,
) []Transition {
	previousByName := make(
		map[string]StatefulSetState,
		len(previous),
	)

	currentByName := make(
		map[string]StatefulSetState,
		len(current),
	)

	for _, statefulSet := range previous {
		previousByName[namespacedName(
			statefulSet.Namespace,
			statefulSet.Name,
		)] = statefulSet
	}

	for _, statefulSet := range current {
		currentByName[namespacedName(
			statefulSet.Namespace,
			statefulSet.Name,
		)] = statefulSet
	}

	var transitions []Transition

	for name, currentState := range currentByName {
		previousState, existed :=
			previousByName[name]

		if !existed {
			transitions = append(
				transitions,
				Transition{
					ResourceType: "statefulset",
					Resource:     name,
					EventType:    EventStateChange,
					Summary: "appeared: " +
						describeStatefulSet(currentState),
				},
			)

			continue
		}

		if previousState == currentState {
			continue
		}

		transitions = append(
			transitions,
			Transition{
				ResourceType: "statefulset",
				Resource:     name,
				EventType:    EventStateChange,
				Summary: fmt.Sprintf(
					"%s -> %s",
					describeStatefulSet(previousState),
					describeStatefulSet(currentState),
				),
			},
		)
	}

	for name := range previousByName {
		if _, exists := currentByName[name]; exists {
			continue
		}

		transitions = append(
			transitions,
			Transition{
				ResourceType: "statefulset",
				Resource:     name,
				EventType:    EventStateChange,
				Summary:      "disappeared",
			},
		)
	}

	return transitions
}

func compareDaemonSets(
	previous []DaemonSetState,
	current []DaemonSetState,
) []Transition {
	previousByName := make(
		map[string]DaemonSetState,
		len(previous),
	)

	currentByName := make(
		map[string]DaemonSetState,
		len(current),
	)

	for _, daemonSet := range previous {
		previousByName[namespacedName(
			daemonSet.Namespace,
			daemonSet.Name,
		)] = daemonSet
	}

	for _, daemonSet := range current {
		currentByName[namespacedName(
			daemonSet.Namespace,
			daemonSet.Name,
		)] = daemonSet
	}

	var transitions []Transition

	for name, currentState := range currentByName {
		previousState, existed :=
			previousByName[name]

		if !existed {
			transitions = append(
				transitions,
				Transition{
					ResourceType: "daemonset",
					Resource:     name,
					EventType:    EventStateChange,
					Summary: "appeared: " +
						describeDaemonSet(currentState),
				},
			)

			continue
		}

		if previousState == currentState {
			continue
		}

		transitions = append(
			transitions,
			Transition{
				ResourceType: "daemonset",
				Resource:     name,
				EventType: classifyHealthTransition(
					daemonSetFailed(previousState),
					daemonSetFailed(currentState),
				),
				Summary: fmt.Sprintf(
					"%s -> %s",
					describeDaemonSet(previousState),
					describeDaemonSet(currentState),
				),
			},
		)
	}

	for name := range previousByName {
		if _, exists := currentByName[name]; exists {
			continue
		}

		transitions = append(
			transitions,
			Transition{
				ResourceType: "daemonset",
				Resource:     name,
				EventType:    EventStateChange,
				Summary:      "disappeared",
			},
		)
	}

	return transitions
}

func compareNodes(
	previous []NodeState,
	current []NodeState,
) []Transition {
	previousByName := make(
		map[string]NodeState,
		len(previous),
	)

	currentByName := make(
		map[string]NodeState,
		len(current),
	)

	for _, node := range previous {
		previousByName[node.Name] = node
	}

	for _, node := range current {
		currentByName[node.Name] = node
	}

	var transitions []Transition

	for name, currentState := range currentByName {
		previousState, existed :=
			previousByName[name]

		if !existed {
			transitions = append(
				transitions,
				Transition{
					ResourceType: "node",
					Resource:     name,
					EventType:    EventStateChange,
					Summary: "appeared: " +
						describeNode(currentState),
				},
			)

			continue
		}

		if previousState == currentState {
			continue
		}

		transitions = append(
			transitions,
			Transition{
				ResourceType: "node",
				Resource:     name,
				EventType: classifyHealthTransition(
					nodeFailed(previousState),
					nodeFailed(currentState),
				),
				Summary: fmt.Sprintf(
					"%s -> %s",
					describeNode(previousState),
					describeNode(currentState),
				),
			},
		)
	}

	for name := range previousByName {
		if _, exists := currentByName[name]; exists {
			continue
		}

		transitions = append(
			transitions,
			Transition{
				ResourceType: "node",
				Resource:     name,
				EventType:    EventStateChange,
				Summary:      "disappeared",
			},
		)
	}

	return transitions
}

func classifyHealthTransition(
	previousFailed bool,
	currentFailed bool,
) string {
	switch {
	case !previousFailed && currentFailed:
		return EventFailure

	case previousFailed && !currentFailed:
		return EventRecovery

	default:
		return EventStateChange
	}
}

func podFailed(pod PodState) bool {
	switch pod.Phase {
	case "Failed", "Unknown":
		return true
	}

	switch pod.Reason {
	case "CrashLoopBackOff",
		"ImagePullBackOff",
		"ErrImagePull",
		"CreateContainerConfigError",
		"CreateContainerError",
		"OOMKilled",
		"Error":
		return true
	}

	return false // Readiness lag alone is not evidence of a failed Pod.
}

func deploymentFailed(
	deployment DeploymentState,
) bool {
	if deployment.Desired == 0 {
		return false
	}

	return deployment.Ready <
		deployment.Desired ||
		deployment.Available <
			deployment.Desired
}

func statefulSetFailed(
	statefulSet StatefulSetState,
) bool {
	if statefulSet.Desired == 0 {
		return false
	}

	return statefulSet.Ready <
		statefulSet.Desired
}

func daemonSetFailed(
	daemonSet DaemonSetState,
) bool {
	if daemonSet.Misscheduled > 0 {
		return true
	}

	return false // Missing replicas during a rollout are not a confirmed incident.
}

func nodeFailed(node NodeState) bool {
	return node.Ready != "True" ||
		node.MemoryPressure ||
		node.DiskPressure ||
		node.PIDPressure
}

func podStatesEqual(
	previous PodState,
	current PodState,
) bool {
	return previous.Phase == current.Phase &&
		previous.Ready == current.Ready &&
		previous.Total == current.Total &&
		previous.Reason == current.Reason
}

func describePod(pod PodState) string {
	result := pod.Phase

	if pod.Reason != "" {
		result += "/" + pod.Reason
	}

	if pod.Total > 0 {
		result += fmt.Sprintf(
			" ready %d/%d",
			pod.Ready,
			pod.Total,
		)
	}

	return result
}

func describeDeployment(
	deployment DeploymentState,
) string {
	return fmt.Sprintf(
		"desired %d updated %d ready %d available %d",
		deployment.Desired,
		deployment.Updated,
		deployment.Ready,
		deployment.Available,
	)
}

func describeStatefulSet(
	statefulSet StatefulSetState,
) string {
	return fmt.Sprintf(
		"desired %d current %d updated %d ready %d",
		statefulSet.Desired,
		statefulSet.Current,
		statefulSet.Updated,
		statefulSet.Ready,
	)
}

func describeDaemonSet(
	daemonSet DaemonSetState,
) string {
	return fmt.Sprintf(
		"desired %d current %d updated %d ready %d available %d misscheduled %d",
		daemonSet.Desired,
		daemonSet.Current,
		daemonSet.Updated,
		daemonSet.Ready,
		daemonSet.Available,
		daemonSet.Misscheduled,
	)
}

func describeNode(node NodeState) string {
	result := "Ready=" + node.Ready

	if node.Unschedulable {
		result += " unschedulable"
	}

	if node.MemoryPressure {
		result += " memory-pressure"
	}

	if node.DiskPressure {
		result += " disk-pressure"
	}

	if node.PIDPressure {
		result += " pid-pressure"
	}

	return result
}

func namespacedName(
	namespace string,
	name string,
) string {
	if namespace == "" {
		return name
	}

	return namespace + "/" + name
}
