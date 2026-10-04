package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	dockercollector "github.com/it-odyssey/waketrail/internal/collectors/docker"
	"github.com/it-odyssey/waketrail/internal/state"
	"github.com/it-odyssey/waketrail/internal/storage"
	watchengine "github.com/it-odyssey/waketrail/internal/watch"
	"github.com/spf13/cobra"
)

var (
	dockerWatchInterval time.Duration
	watchDetach         bool
	watchWorker         bool
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Continuously watch external system state",
	Args:  cobra.NoArgs,

	RunE: func(cmd *cobra.Command, args []string) error {
		return startDockerWatch(watchDetach)
	},
}

var watchDockerCmd = &cobra.Command{
	Use:   "docker",
	Short: "Continuously watch Docker container state",
	Args:  cobra.NoArgs,

	RunE: func(cmd *cobra.Command, args []string) error {
		return startDockerWatch(watchDetach)
	},
}

var watchStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show continuous watch status",
	Args:  cobra.NoArgs,

	RunE: func(cmd *cobra.Command, args []string) error {
		watch, err := state.LoadWatchState()

		if errors.Is(err, state.ErrWatchNotRunning) {
			fmt.Println("No WakeTrail watcher is running.")
			return nil
		}

		if err != nil {
			return err
		}

		if !processRunning(watch.PID) {
			_ = state.ClearWatchState()

			fmt.Println(
				"No WakeTrail watcher is running. Cleared stale watch state.",
			)

			return nil
		}

		fmt.Printf(
			"Watcher running\nPID: %d\nCollector: %s\nSession ID: %d\nStarted: %s\n",
			watch.PID,
			watch.Collector,
			watch.SessionID,
			watch.StartedAt.Format("2006-01-02 15:04:05"),
		)

		return nil
	},
}

var watchStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the continuous watcher",
	Args:  cobra.NoArgs,

	RunE: func(cmd *cobra.Command, args []string) error {
		watch, err := state.LoadWatchState()

		if errors.Is(err, state.ErrWatchNotRunning) {
			fmt.Println("No WakeTrail watcher is running.")
			return nil
		}

		if err != nil {
			return err
		}

		process, err := os.FindProcess(watch.PID)
		if err != nil {
			return err
		}

		if err := process.Signal(syscall.SIGTERM); err != nil {
			if !processRunning(watch.PID) {
				_ = state.ClearWatchState()
				fmt.Println("Watcher was already stopped.")
				return nil
			}

			return err
		}

		fmt.Println("Stopping WakeTrail watcher.")

		return nil
	},
}

func startDockerWatch(detach bool) error {
	collector := dockercollector.NewCollector()

	return startCollectorWatch(
		collector,
		detach,
	)
}

func startCollectorWatch(
	collector watchengine.Collector,
	detach bool,
) error {
	active, err := state.HasActiveSession()
	if err != nil {
		return err
	}

	if !active {
		return fmt.Errorf(
			"no active WakeTrail session; start one before watching",
		)
	}

	// The detached worker was already validated by the parent process.
	// It must not reject itself when it sees its own watch state.
	if !watchWorker {
		if err := ensureNoActiveWatch(); err != nil {
			return err
		}
	}

	session, err := state.LoadSession()
	if err != nil {
		return err
	}

	if detach && !watchWorker {
		return startDetachedDockerWatch(session.ID)
	}

	return runCollectorWatch(
		session.ID,
		collector,
	)
}

func startDetachedDockerWatch(sessionID int64) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}

	child := exec.Command(
		executable,
		"watch",
		"docker",
		"--worker",
		"--interval",
		dockerWatchInterval.String(),
	)

	child.Stdin = nil
	child.Stdout = nil
	child.Stderr = nil

	child.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	if err := child.Start(); err != nil {
		return err
	}

	pid := child.Process.Pid

	if err := state.SaveWatchState(state.WatchState{
		PID:       pid,
		SessionID: sessionID,
		Collector: "docker",
		StartedAt: time.Now(),
	}); err != nil {
		_ = child.Process.Kill()
		return err
	}

	if err := child.Process.Release(); err != nil {
		return err
	}

	fmt.Printf(
		"WakeTrail watcher started in background. PID: %d\n",
		pid,
	)

	return nil
}

func runCollectorWatch(
	sessionID int64,
	collector watchengine.Collector,
) error {
	previous, err := collector.Snapshot()
	if err != nil {
		return err
	}

	if !watchWorker {
		if err := state.SaveWatchState(state.WatchState{
			PID:       os.Getpid(),
			SessionID: sessionID,
			Collector: collector.Name(),
			StartedAt: time.Now(),
		}); err != nil {
			return err
		}
	}

	defer state.ClearWatchState()

	store, err := storage.Open()
	if err != nil {
		return err
	}
	defer store.Close()

	if !watchWorker {
		fmt.Printf(
			"Watching %s every %s. Press Ctrl+C to stop.\n",
			collector.Name(),
			dockerWatchInterval,
		)

		fmt.Println("Baseline captured.")
	}

	ticker := time.NewTicker(dockerWatchInterval)
	defer ticker.Stop()

	signals := make(chan os.Signal, 1)

	signal.Notify(
		signals,
		os.Interrupt,
		syscall.SIGTERM,
	)

	defer signal.Stop(signals)

	for {
		select {
		case <-ticker.C:
			current, err := collector.Snapshot()
			if err != nil {
				if !watchWorker {
					fmt.Printf(
						"%s observation failed: %v\n",
						collector.Name(),
						err,
					)
				}

				continue
			}

			events, err := collector.Compare(
				previous,
				current,
			)
			if err != nil {
				if !watchWorker {
					fmt.Printf(
						"%s comparison failed: %v\n",
						collector.Name(),
						err,
					)
				}

				continue
			}

			for _, event := range events {
				timelineEvent := storage.TimelineEvent{
					SessionID:  &sessionID,
					EventType:  event.EventType,
					Source:     event.Source,
					Summary:    event.Summary,
					OccurredAt: time.Now(),
				}

				if _, err := store.InsertTimelineEvent(
					timelineEvent,
				); err != nil {
					return err
				}

				if !watchWorker {
					fmt.Printf(
						"[%s] %s: %s\n",
						event.EventType,
						event.Source,
						event.Summary,
					)
				}
			}

			previous = current

		case <-signals:
			if !watchWorker {
				fmt.Println()
				fmt.Printf(
					"%s watch stopped.\n",
					collector.Name(),
				)
			}

			return nil
		}
	}
}

func ensureNoActiveWatch() error {
	watch, err := state.LoadWatchState()

	if errors.Is(err, state.ErrWatchNotRunning) {
		return nil
	}

	if err != nil {
		return err
	}

	if processRunning(watch.PID) {
		return fmt.Errorf(
			"a WakeTrail watcher is already running with PID %d",
			watch.PID,
		)
	}

	return state.ClearWatchState()
}

func processRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	return process.Signal(syscall.Signal(0)) == nil
}

func init() {
	watchCmd.Flags().BoolVarP(
		&watchDetach,
		"detach",
		"d",
		false,
		"run watcher in the background",
	)

	watchDockerCmd.Flags().BoolVarP(
		&watchDetach,
		"detach",
		"d",
		false,
		"run watcher in the background",
	)

	watchDockerCmd.Flags().BoolVar(
		&watchWorker,
		"worker",
		false,
		"run internal detached watcher",
	)

	_ = watchDockerCmd.Flags().MarkHidden("worker")

	watchDockerCmd.Flags().DurationVar(
		&dockerWatchInterval,
		"interval",
		2*time.Second,
		"interval between Docker observations",
	)

	watchCmd.Flags().DurationVar(
		&dockerWatchInterval,
		"interval",
		2*time.Second,
		"interval between observations",
	)

	watchCmd.AddCommand(
		watchDockerCmd,
		watchStatusCmd,
		watchStopCmd,
	)

	rootCmd.AddCommand(watchCmd)
}
