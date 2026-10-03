package cmd

import (
	"os"
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
	recordStdoutFile  string
	recordStderrFile  string
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

		if recordStdoutFile != "" || recordStderrFile != "" {
			output, err := loadCommandOutput(
				commandEventID,
				recordCaptureMode,
				recordStdoutFile,
				recordStderrFile,
			)
			if err != nil {
				return err
			}

			if err := store.InsertCommandOutput(output); err != nil {
				return err
			}
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

func readCapturedFile(
	path string,
	maxBytes int64,
) (string, int64, bool, error) {
	if path == "" {
		return "", 0, false, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", 0, false, err
	}

	totalBytes := info.Size()
	truncated := false

	if maxBytes > 0 && totalBytes > maxBytes {
		file, err := os.Open(path)
		if err != nil {
			return "", 0, false, err
		}
		defer file.Close()

		data := make([]byte, maxBytes)

		n, err := file.Read(data)
		if err != nil {
			return "", 0, false, err
		}

		data = data[:n]
		truncated = true

		return string(data), totalBytes, truncated, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", 0, false, err
	}

	return string(data), totalBytes, truncated, nil
}

func loadCommandOutput(
	commandEventID int64,
	captureMode string,
	stdoutPath string,
	stderrPath string,
) (storage.CommandOutput, error) {
	const boundedLimit int64 = 64 * 1024

	var maxBytes int64

	if captureMode == "bounded" {
		maxBytes = boundedLimit
	}

	stdout, stdoutBytes, stdoutTruncated, err := readCapturedFile(
		stdoutPath,
		maxBytes,
	)
	if err != nil {
		return storage.CommandOutput{}, err
	}

	stderr, stderrBytes, stderrTruncated, err := readCapturedFile(
		stderrPath,
		maxBytes,
	)
	if err != nil {
		return storage.CommandOutput{}, err
	}

	output := storage.CommandOutput{
		CommandEventID:  commandEventID,
		Stdout:          stdout,
		Stderr:          stderr,
		StdoutBytes:     stdoutBytes,
		StderrBytes:     stderrBytes,
		StdoutTruncated: stdoutTruncated,
		StderrTruncated: stderrTruncated,
	}

	return output, nil
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

	recordCmd.Flags().StringVar(
		&recordStdoutFile,
		"stdout-file",
		"",
		"path to captured stdout",
	)

	recordCmd.Flags().StringVar(
		&recordStderrFile,
		"stderr-file",
		"",
		"path to captured stderr",
	)
}
