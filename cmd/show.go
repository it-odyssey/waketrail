package cmd

import (
	"errors"
	"fmt"
	"time"

	"github.com/it-odyssey/waketrail/internal/storage"
	"github.com/spf13/cobra"
)

type displayEvent struct {
	OccurredAt time.Time
	Kind       string

	CommandEvent      *storage.CommandEvent
	TimelineEvent     *storage.TimelineEvent
	CorrelatedCommand *storage.CommandEvent
	Activity          *displayActivity
}

var showVerbose bool

var showCmd = &cobra.Command{
	Use:   "show [session-name]",
	Short: "Show a WakeTrail session timeline",
	Args:  cobra.MaximumNArgs(1),

	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := storage.Open()
		if err != nil {
			return err
		}
		defer store.Close()

		var session storage.SessionRecord

		if len(args) == 1 {
			session, err = store.SessionByName(args[0])
		} else {
			session, err = store.LatestSession()
		}

		if errors.Is(err, storage.ErrSessionNotFound) {
			fmt.Println("No matching WakeTrail session found.")
			return nil
		}

		if err != nil {
			return err
		}

		commandEvents, err := store.CommandEventsForSession(session.ID)
		if err != nil {
			return err
		}

		timelineEvents, err := store.TimelineEventsForSession(session.ID)
		if err != nil {
			return err
		}

		events := buildDisplayEvents(commandEvents, timelineEvents, showVerbose)

		printReport(
			store,
			session,
			events,
			commandEvents,
			timelineEvents,
		)

		return nil
	},
}

func init() {
	showCmd.Flags().BoolVar(
		&showVerbose,
		"verbose",
		false,
		"show additional event metadata",
	)

	rootCmd.AddCommand(showCmd)
}
