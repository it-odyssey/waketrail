package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	dockercollector "github.com/it-odyssey/waketrail/internal/collectors/docker"
	"github.com/it-odyssey/waketrail/internal/state"
	"github.com/it-odyssey/waketrail/internal/storage"
	"github.com/spf13/cobra"
)

var dockerWatchInterval time.Duration

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Continuously watch external system state",
}

var watchDockerCmd = &cobra.Command{
	Use:   "docker",
	Short: "Continuously watch Docker container state",

	RunE: func(cmd *cobra.Command, args []string) error {
		active, err := state.HasActiveSession()
		if err != nil {
			return err
		}

		if !active {
			return fmt.Errorf(
				"no active WakeTrail session; start one before watching Docker",
			)
		}

		session, err := state.LoadSession()
		if err != nil {
			return err
		}

		previous, err := dockercollector.Detect()
		if err != nil {
			return err
		}

		store, err := storage.Open()
		if err != nil {
			return err
		}
		defer store.Close()

		fmt.Printf(
			"Watching Docker every %s. Press Ctrl+C to stop.\n",
			dockerWatchInterval,
		)

		fmt.Printf(
			"Baseline: %d containers\n",
			len(previous),
		)

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
				current, err := dockercollector.Detect()
				if err != nil {
					fmt.Printf(
						"Docker observation failed: %v\n",
						err,
					)
					continue
				}

				transitions := dockercollector.Compare(
					previous,
					current,
				)

				for _, transition := range transitions {
					event := storage.TimelineEvent{
						SessionID:  &session.ID,
						EventType:  transition.EventType,
						Source:     "docker",
						Summary:    transition.Summary,
						OccurredAt: time.Now(),
					}

					if _, err := store.InsertTimelineEvent(
						event,
					); err != nil {
						return err
					}

					fmt.Printf(
						"[%s] %s: %s\n",
						event.EventType,
						event.Source,
						event.Summary,
					)
				}

				previous = current

			case <-signals:
				fmt.Println()
				fmt.Println("Docker watch stopped.")
				return nil
			}
		}
	},
}

func init() {
	watchDockerCmd.Flags().DurationVar(
		&dockerWatchInterval,
		"interval",
		2*time.Second,
		"interval between Docker observations",
	)

	watchCmd.AddCommand(watchDockerCmd)
	rootCmd.AddCommand(watchCmd)
}
