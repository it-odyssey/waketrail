package cmd

import (
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/it-odyssey/waketrail/internal/storage"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List recorded WakeTrail sessions",

	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := storage.Open()
		if err != nil {
			return err
		}
		defer store.Close()

		sessions, err := store.ListSessions()
		if err != nil {
			return err
		}

		if len(sessions) == 0 {
			fmt.Println("No WakeTrail sessions found.")
			return nil
		}

		writer := tabwriter.NewWriter(
			cmd.OutOrStdout(),
			0,
			4,
			3,
			' ',
			0,
		)
		defer writer.Flush()

		fmt.Fprintln(
			writer,
			"ID\tSESSION\tSTARTED\tDURATION\tEVENTS",
		)

		for _, session := range sessions {
			duration := "recording"

			if session.EndedAt != nil {
				duration = session.EndedAt.
					Sub(session.StartedAt).
					Round(time.Second).
					String()
			}

			fmt.Fprintf(
				writer,
				"%d\t%s\t%s\t%s\t%d\n",
				session.ID,
				session.Name,
				session.StartedAt.Format(
					"2006-01-02 15:04",
				),
				duration,
				session.EventCount,
			)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
