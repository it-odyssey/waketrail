package cmd

import (
	"fmt"
	"time"

	"github.com/it-odyssey/waketrail/internal/redact"
	"github.com/it-odyssey/waketrail/internal/state"
	"github.com/it-odyssey/waketrail/internal/storage"
	"github.com/spf13/cobra"
)

var markCmd = &cobra.Command{
	Use:   "mark [note]",
	Short: "Add a note to the active WakeTrail session",
	Args:  cobra.ExactArgs(1),

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

		event := storage.TimelineEvent{
			SessionID:  &session.ID,
			EventType:  "note",
			Source:     "user",
			Summary:    redact.String(args[0]),
			OccurredAt: time.Now(),
		}

		if _, err := store.InsertTimelineEvent(event); err != nil {
			return err
		}

		fmt.Printf("Added note: %s\n", event.Summary)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(markCmd)
}
