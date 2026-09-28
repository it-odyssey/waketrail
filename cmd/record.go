package cmd

import (
	"strconv"
	"time"

	gitcollector "github.com/it-odyssey/waketrail/internal/collectors/git"
	"github.com/it-odyssey/waketrail/internal/state"
	"github.com/it-odyssey/waketrail/internal/storage"
	"github.com/spf13/cobra"
)

var (
	recordCwd         string
	recordExitCode    int
	recordCaptureMode string
	recordStartedAt   int64
	recordEndedAt     int64
)

var recordCmd = &cobra.Command{
	Use:    "record [command]",
	Short:  "Record a shell command",
	Hidden: true,
	Args:   cobra.ExactArgs(1),

	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := storage.Open()
		if err != nil {
			return err
		}
		defer store.Close()

		var sessionID *int64

		active, err := state.HasActiveSession()
		if err != nil {
			return err
		}

		if active {
			session, err := state.LoadSession()
			if err != nil {
				return err
			}

			sessionID = &session.ID
		}

		event := storage.CommandEvent{
			SessionID:   sessionID,
			Command:     args[0],
			Cwd:         recordCwd,
			ExitCode:    recordExitCode,
			CaptureMode: recordCaptureMode,
			StartedAt:   time.Unix(0, recordStartedAt),
			EndedAt:     time.Unix(0, recordEndedAt),
		}

		commandEventID, err := store.InsertCommandEvent(event)
		if err != nil {
			return err
		}

		gitContext, err := gitcollector.Detect(recordCwd)
		if err != nil {
			return err
		}

		if gitContext.IsRepository {
			err := store.InsertGitContext(storage.GitContext{
				CommandEventID: commandEventID,
				RepositoryRoot: gitContext.Root,
				Branch:         gitContext.Branch,
				CommitSHA:      gitContext.Commit,
				Dirty:          gitContext.Dirty,
			})
			if err != nil {
				return err
			}
		}

		return nil
	},
}

func init() {
	recordCmd.Flags().StringVar(
		&recordCwd,
		"cwd",
		"",
		"Working directory",
	)

	recordCmd.Flags().StringVar(
		&recordCaptureMode,
		"capture-mode",
		"none",
		"command output capture mode",
	)

	rootCmd.AddCommand(recordCmd)

	recordCmd.Flags().IntVar(
		&recordExitCode,
		"exit-code",
		0,
		"Command exit code",
	)

	recordCmd.Flags().Func(
		"started-at",
		"Command start time in Unix nanoseconds",
		func(value string) error {
			parsed, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return err
			}

			recordStartedAt = parsed
			return nil
		},
	)

	recordCmd.Flags().Func(
		"ended-at",
		"Command end time in Unix nanoseconds",
		func(value string) error {
			parsed, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return err
			}

			recordEndedAt = parsed
			return nil
		},
	)
}
