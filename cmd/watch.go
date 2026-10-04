package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	dockercollector "github.com/it-odyssey/waketrail/internal/collectors/docker"
	kubernetescollector "github.com/it-odyssey/waketrail/internal/collectors/kubernetes"
	systemdcollector "github.com/it-odyssey/waketrail/internal/collectors/systemd"
	"github.com/it-odyssey/waketrail/internal/state"
	"github.com/it-odyssey/waketrail/internal/storage"
	watchengine "github.com/it-odyssey/waketrail/internal/watch"
	"github.com/spf13/cobra"
)

var (
	watchInterval time.Duration
	watchDetach   bool
	watchWorker   bool
)

type activeCollector struct {
	collector watchengine.Collector
	previous  any
}

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Continuously watch external system state",
	Args:  cobra.NoArgs,

	RunE: func(cmd *cobra.Command, args []string) error {
		return startCollectorsWatch(
			allWatchCollectors(),
			watchDetach,
			"",
		)
	},
}

var watchDockerCmd = &cobra.Command{
	Use:   "docker",
	Short: "Continuously watch Docker container state",
	Args:  cobra.NoArgs,

	RunE: func(cmd *cobra.Command, args []string) error {
		return startCollectorsWatch(
			[]watchengine.Collector{
				dockercollector.NewCollector(),
			},
			watchDetach,
			"docker",
		)
	},
}

var watchSystemdCmd = &cobra.Command{
	Use:   "systemd",
	Short: "Continuously watch systemd service state",
	Args:  cobra.NoArgs,

	RunE: func(cmd *cobra.Command, args []string) error {
		return startCollectorsWatch(
			[]watchengine.Collector{
				systemdcollector.NewCollector(),
			},
			watchDetach,
			"systemd",
		)
	},
}

var watchKubernetesCmd = &cobra.Command{
	Use:   "kubernetes",
	Short: "Continuously watch Kubernetes resource state",
	Args:  cobra.NoArgs,

	RunE: func(cmd *cobra.Command, args []string) error {
		return startCollectorsWatch(
			[]watchengine.Collector{
				kubernetescollector.NewCollector(),
			},
			watchDetach,
			"kubernetes",
		)
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
			"Watcher running\nPID: %d\nCollectors: %s\nSession ID: %d\nStarted: %s\n",
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

func allWatchCollectors() []watchengine.Collector {
	return []watchengine.Collector{
		dockercollector.NewCollector(),
		systemdcollector.NewCollector(),
		kubernetescollector.NewCollector(),
	}
}

func startCollectorsWatch(
	collectors []watchengine.Collector,
	detach bool,
	target string,
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

	// Detached workers have already been validated by the parent process.
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
		return startDetachedWatch(
			session.ID,
			collectors,
			target,
		)
	}

	return runCollectorsWatch(
		session.ID,
		collectors,
	)
}

func startDetachedWatch(
	sessionID int64,
	collectors []watchengine.Collector,
	target string,
) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}

	args := []string{"watch"}

	if target != "" {
		args = append(args, target)
	}

	args = append(
		args,
		"--worker",
		"--interval",
		watchInterval.String(),
	)

	child := exec.Command(
		executable,
		args...,
	)

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	logDir := filepath.Join(
		home,
		".local",
		"state",
		"waketrail",
	)

	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	logPath := filepath.Join(
		logDir,
		"watch.log",
	)

	logFile, err := os.OpenFile(
		logPath,
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644,
	)
	if err != nil {
		return err
	}

	defer logFile.Close()

	child.Stdin = nil
	child.Stdout = logFile
	child.Stderr = logFile

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
		Collector: collectorNames(collectors),
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

func runCollectorsWatch(
	sessionID int64,
	collectors []watchengine.Collector,
) error {
	activeCollectors := make(
		[]activeCollector,
		0,
		len(collectors),
	)

	for _, collector := range collectors {
		snapshot, err := collector.Snapshot()
		if err != nil {
			if !watchWorker {
				fmt.Printf(
					"Skipping %s collector: %v\n",
					collector.Name(),
					err,
				)
			}

			continue
		}

		activeCollectors = append(
			activeCollectors,
			activeCollector{
				collector: collector,
				previous:  snapshot,
			},
		)
	}

	if len(activeCollectors) == 0 {
		return fmt.Errorf(
			"no supported collectors are available",
		)
	}

	if !watchWorker {
		if err := state.SaveWatchState(state.WatchState{
			PID:       os.Getpid(),
			SessionID: sessionID,
			Collector: activeCollectorNames(activeCollectors),
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
			activeCollectorNames(activeCollectors),
			watchInterval,
		)

		fmt.Println("Baselines captured.")
	}

	ticker := time.NewTicker(watchInterval)
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
			for i := range activeCollectors {
				active := &activeCollectors[i]

				current, err := active.collector.Snapshot()
				if err != nil {
					if !watchWorker {
						fmt.Printf(
							"%s observation failed: %v\n",
							active.collector.Name(),
							err,
						)
					}

					continue
				}

				events, err := active.collector.Compare(
					active.previous,
					current,
				)
				if err != nil {
					if !watchWorker {
						fmt.Printf(
							"%s comparison failed: %v\n",
							active.collector.Name(),
							err,
						)
					}

					continue
				}

				for _, event := range events {
					timelineEvent := storage.TimelineEvent{
						SessionID:    &sessionID,
						EventType:    event.EventType,
						Source:       event.Source,
						ResourceType: event.ResourceType,
						Resource:     event.Resource,
						Summary:      event.Summary,
						OccurredAt:   time.Now(),
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

				active.previous = current
			}

		case <-signals:
			if !watchWorker {
				fmt.Println()
				fmt.Println("WakeTrail watch stopped.")
			}

			return nil
		}
	}
}

func collectorNames(
	collectors []watchengine.Collector,
) string {
	names := make(
		[]string,
		0,
		len(collectors),
	)

	for _, collector := range collectors {
		names = append(
			names,
			collector.Name(),
		)
	}

	return strings.Join(names, ", ")
}

func activeCollectorNames(
	collectors []activeCollector,
) string {
	names := make(
		[]string,
		0,
		len(collectors),
	)

	for _, active := range collectors {
		names = append(
			names,
			active.collector.Name(),
		)
	}

	return strings.Join(names, ", ")
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

func addWatchFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(
		&watchDetach,
		"detach",
		"d",
		false,
		"run watcher in the background",
	)

	cmd.Flags().BoolVar(
		&watchWorker,
		"worker",
		false,
		"run internal detached watcher",
	)

	_ = cmd.Flags().MarkHidden("worker")

	cmd.Flags().DurationVar(
		&watchInterval,
		"interval",
		2*time.Second,
		"interval between observations",
	)
}

func init() {
	addWatchFlags(watchCmd)
	addWatchFlags(watchDockerCmd)
	addWatchFlags(watchSystemdCmd)
	addWatchFlags(watchKubernetesCmd)

	watchCmd.AddCommand(
		watchDockerCmd,
		watchSystemdCmd,
		watchKubernetesCmd,
		watchStatusCmd,
		watchStopCmd,
	)

	rootCmd.AddCommand(watchCmd)
}
