package systemd

import "testing"

func TestCompareDetectsStoppedService(t *testing.T) {
	previous := []UnitState{
		{
			Name:   "nginx.service",
			Active: "active",
			Sub:    "running",
		},
	}

	current := []UnitState{
		{
			Name:   "nginx.service",
			Active: "inactive",
			Sub:    "dead",
		},
	}

	transitions := Compare(previous, current)

	if len(transitions) != 1 {
		t.Fatalf(
			"len(transitions) = %d, want 1",
			len(transitions),
		)
	}

	if transitions[0].EventType != EventStopped {
		t.Errorf(
			"EventType = %q, want %q",
			transitions[0].EventType,
			EventStopped,
		)
	}
}

func TestCompareDetectsStartedService(t *testing.T) {
	previous := []UnitState{
		{
			Name:   "nginx.service",
			Active: "inactive",
			Sub:    "dead",
		},
	}

	current := []UnitState{
		{
			Name:   "nginx.service",
			Active: "active",
			Sub:    "running",
		},
	}

	transitions := Compare(previous, current)

	if len(transitions) != 1 {
		t.Fatalf(
			"len(transitions) = %d, want 1",
			len(transitions),
		)
	}

	if transitions[0].EventType != EventStarted {
		t.Errorf(
			"EventType = %q, want %q",
			transitions[0].EventType,
			EventStarted,
		)
	}
}

func TestCompareDetectsFailedService(t *testing.T) {
	previous := []UnitState{
		{
			Name:   "api.service",
			Active: "active",
			Sub:    "running",
		},
	}

	current := []UnitState{
		{
			Name:   "api.service",
			Active: "failed",
			Sub:    "failed",
		},
	}

	transitions := Compare(previous, current)

	if len(transitions) != 1 {
		t.Fatalf(
			"len(transitions) = %d, want 1",
			len(transitions),
		)
	}

	if transitions[0].EventType != EventFailure {
		t.Errorf(
			"EventType = %q, want %q",
			transitions[0].EventType,
			EventFailure,
		)
	}
}

func TestCompareDetectsRecovery(t *testing.T) {
	previous := []UnitState{
		{
			Name:   "api.service",
			Active: "failed",
			Sub:    "failed",
		},
	}

	current := []UnitState{
		{
			Name:   "api.service",
			Active: "active",
			Sub:    "running",
		},
	}

	transitions := Compare(previous, current)

	if len(transitions) != 1 {
		t.Fatalf(
			"len(transitions) = %d, want 1",
			len(transitions),
		)
	}

	if transitions[0].EventType != EventRecovery {
		t.Errorf(
			"EventType = %q, want %q",
			transitions[0].EventType,
			EventRecovery,
		)
	}
}
