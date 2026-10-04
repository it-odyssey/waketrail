package state

import (
	"errors"
	"testing"
	"time"
)

func TestWatchStateSaveLoadAndClear(t *testing.T) {
	t.Setenv(
		"XDG_STATE_HOME",
		t.TempDir(),
	)

	expected := WatchState{
		PID:       12345,
		SessionID: 42,
		Collector: "docker",
		StartedAt: time.Now().
			Truncate(time.Second),
	}

	if err := SaveWatchState(expected); err != nil {
		t.Fatalf(
			"SaveWatchState() returned error: %v",
			err,
		)
	}

	actual, err := LoadWatchState()
	if err != nil {
		t.Fatalf(
			"LoadWatchState() returned error: %v",
			err,
		)
	}

	if actual.PID != expected.PID {
		t.Errorf(
			"PID = %d, want %d",
			actual.PID,
			expected.PID,
		)
	}

	if actual.SessionID != expected.SessionID {
		t.Errorf(
			"SessionID = %d, want %d",
			actual.SessionID,
			expected.SessionID,
		)
	}

	if actual.Collector != expected.Collector {
		t.Errorf(
			"Collector = %q, want %q",
			actual.Collector,
			expected.Collector,
		)
	}

	if err := ClearWatchState(); err != nil {
		t.Fatalf(
			"ClearWatchState() returned error: %v",
			err,
		)
	}

	_, err = LoadWatchState()

	if !errors.Is(err, ErrWatchNotRunning) {
		t.Fatalf(
			"LoadWatchState() error = %v, want %v",
			err,
			ErrWatchNotRunning,
		)
	}
}
