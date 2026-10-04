package cmd

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/it-odyssey/waketrail/internal/state"
	"github.com/it-odyssey/waketrail/internal/storage"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the current WakeTrail recording session",
	RunE: func(cmd *cobra.Command, args []string) error {
		active, err := state.HasActiveSession()
		if err != nil {
			return err
		}

		if !active {
			fmt.Println("No active recording.")
			return nil
		}

		session, err := state.LoadSession()
		if err != nil {
			return err
		}

		store, err := storage.Open()
		if err != nil {
			return err
		}
		defer store.Close()

		if err := stopSessionWatcher(session.ID); err != nil {
			return err
		}

		endedAt := time.Now()

		if err := store.EndSession(session.ID, endedAt); err != nil {
			return err
		}

		duration := endedAt.Sub(session.StartedAt)

		if err := state.ClearSession(); err != nil {
			return err
		}

		fmt.Printf("Stopped recording: %s\n", session.Name)
		fmt.Printf("Duration: %s\n", duration.Round(time.Second))

		return nil
	},
}

func stopSessionWatcher(sessionID int64) error {
	watch, err := state.LoadWatchState()

	if errors.Is(err, state.ErrWatchNotRunning) {
		return nil
	}

	if err != nil {
		return err
	}

	// Do not stop a watcher that belongs to some other session.
	if watch.SessionID != sessionID {
		return nil
	}

	if !processRunning(watch.PID) {
		return state.ClearWatchState()
	}

	process, err := os.FindProcess(watch.PID)
	if err != nil {
		return err
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		if !processRunning(watch.PID) {
			return state.ClearWatchState()
		}

		return err
	}

	// Give the worker a brief opportunity to handle SIGTERM and
	// clear its own watch state before the recording is closed.
	deadline := time.Now().Add(2 * time.Second)

	for processRunning(watch.PID) &&
		time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}

	if processRunning(watch.PID) {
		return fmt.Errorf(
			"watcher PID %d did not stop",
			watch.PID,
		)
	}

	return state.ClearWatchState()
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
