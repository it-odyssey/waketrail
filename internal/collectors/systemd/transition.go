package systemd

import "fmt"

const (
	EventStateChange = "state_change"
	EventFailure     = "failure"
	EventRecovery    = "recovery"
	EventStopped     = "stopped"
	EventStarted     = "started"
)

type Transition struct {
	Name      string
	EventType string
	Previous  UnitState
	Current   UnitState
	Summary   string
}

func Compare(
	previous []UnitState,
	current []UnitState,
) []Transition {
	previousByName := unitMap(previous)
	currentByName := unitMap(current)

	var transitions []Transition

	for name, currentState := range currentByName {
		previousState, existed := previousByName[name]

		if !existed {
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

	return transitions
}

func unitMap(
	units []UnitState,
) map[string]UnitState {
	result := make(map[string]UnitState, len(units))

	for _, unit := range units {
		result[unit.Name] = unit
	}

	return result
}

func statesEqual(
	previous UnitState,
	current UnitState,
) bool {
	return previous.Active == current.Active &&
		previous.Sub == current.Sub
}

func classifyTransition(
	previous UnitState,
	current UnitState,
) string {
	previousFailed := isFailureState(previous)
	currentFailed := isFailureState(current)

	switch {
	case !previousFailed && currentFailed:
		return EventFailure

	case previousFailed && !currentFailed:
		return EventRecovery

	case previous.Active == "active" &&
		current.Active == "inactive":
		return EventStopped

	case previous.Active == "inactive" &&
		current.Active == "active":
		return EventStarted

	default:
		return EventStateChange
	}
}

func isFailureState(unit UnitState) bool {
	return unit.Active == "failed" ||
		unit.Sub == "failed"
}

func describeState(unit UnitState) string {
	if unit.Sub == "" ||
		unit.Sub == unit.Active {
		return unit.Active
	}

	return unit.Active + "/" + unit.Sub
}
