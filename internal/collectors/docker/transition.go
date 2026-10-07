package docker

import (
	"fmt"
	"strings"
)

const (
	EventStateChange = "state_change"
	EventFailure     = "failure"
	EventRecovery    = "recovery"
	EventStopped     = "stopped"
	EventStarted     = "started"
	EventCreated     = "created"
	EventRemoved     = "removed"
)

type Transition struct {
	Name      string
	EventType string
	Previous  ContainerState
	Current   ContainerState
	Summary   string
}

func Compare(
	previous []ContainerState,
	current []ContainerState,
) []Transition {
	previousByName := containerMap(previous)
	currentByName := containerMap(current)

	var transitions []Transition

	for name, currentState := range currentByName {
		previousState, existed := previousByName[name]

		if !existed {
			transitions = append(transitions, Transition{
				Name:      name,
				EventType: EventCreated,
				Current:   currentState,
				Summary: fmt.Sprintf(
					"%s appeared: %s",
					name,
					describeState(currentState),
				),
			})

			continue
		}

		if statesEqual(previousState, currentState) {
			continue
		}

		eventType := classifyTransition(
			previousState,
			currentState,
		)

		transitions = append(transitions, Transition{
			Name:      name,
			EventType: eventType,
			Previous:  previousState,
			Current:   currentState,
			Summary: fmt.Sprintf(
				"%s: %s -> %s",
				name,
				describeState(previousState),
				describeState(currentState),
			),
		})
	}

	for name, previousState := range previousByName {
		if _, exists := currentByName[name]; exists {
			continue
		}

		transitions = append(transitions, Transition{
			Name:      name,
			EventType: EventRemoved,
			Previous:  previousState,
			Summary: fmt.Sprintf(
				"%s disappeared",
				name,
			),
		})
	}

	return transitions
}

func containerMap(
	containers []ContainerState,
) map[string]ContainerState {
	result := make(map[string]ContainerState, len(containers))

	for _, container := range containers {
		result[container.Name] = container
	}

	return result
}

func statesEqual(
	previous ContainerState,
	current ContainerState,
) bool {
	return previous.State == current.State &&
		previous.Health == current.Health
}

func classifyTransition(
	previous ContainerState,
	current ContainerState,
) string {
	// A clean Docker exit usually means the container was intentionally
	// stopped rather than crashing.
	if previous.State == "running" &&
		current.State == "exited" &&
		isCleanExit(current) {
		return EventStopped
	}

	// A container returning from a clean stopped state is a normal start,
	// not a recovery from failure.
	if previous.State == "exited" &&
		isCleanExit(previous) &&
		current.State == "running" {
		return EventStarted
	}

	previousFailed := isFailureState(previous)
	currentFailed := isFailureState(current)

	switch {
	case !previousFailed && currentFailed:
		return EventFailure

	case previousFailed && !currentFailed:
		return EventRecovery

	default:
		return EventStateChange
	}
}

func isFailureState(container ContainerState) bool {
	if container.Health == "unhealthy" {
		return true
	}

	switch container.State {
	case "dead", "restarting":
		return true

	case "exited":
		return !isCleanExit(container)

	default:
		return false
	}
}

func isCleanExit(container ContainerState) bool {
	return strings.Contains(
		container.Status,
		"Exited (0)",
	)
}

func describeState(container ContainerState) string {
	if container.Health != "" {
		return fmt.Sprintf(
			"%s/%s",
			container.State,
			container.Health,
		)
	}

	if container.State == "" {
		return "unknown"
	}

	if container.State == "exited" && container.Status != "" {
		if isCleanExit(container) {
			return "exited (0)"
		}
	}

	return container.State
}
