package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/it-odyssey/waketrail/internal/capture"
	gitcollector "github.com/it-odyssey/waketrail/internal/collectors/git"
	terraformcollector "github.com/it-odyssey/waketrail/internal/collectors/terraform"
	"github.com/it-odyssey/waketrail/internal/redact"
	"github.com/it-odyssey/waketrail/internal/state"
	"github.com/it-odyssey/waketrail/internal/storage"
	"github.com/spf13/cobra"
)

var (
	recordCwd           string
	recordExitCode      int
	recordCaptureMode   string
	recordStartedAt     int64
	recordEndedAt       int64
	recordStdoutFile    string
	recordStderrFile    string
	recordGitBeforeFile string
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

		captureMode := capture.RecordingMode(args[0], recordCaptureMode)
		event := storage.CommandEvent{
			SessionID:   sessionID,
			Command:     redact.Command(args[0]),
			Cwd:         recordCwd,
			ExitCode:    recordExitCode,
			CaptureMode: string(captureMode),
			StartedAt:   time.Unix(0, recordStartedAt),
			EndedAt:     time.Unix(0, recordEndedAt),
		}

		commandEventID, err := store.InsertCommandEvent(event)
		if err != nil {
			return err
		}

		var commandOutput *storage.CommandOutput

		if captureMode != capture.ModeNone && (recordStdoutFile != "" || recordStderrFile != "") {
			output, err := loadCommandOutput(
				commandEventID,
				string(captureMode),
				recordStdoutFile,
				recordStderrFile,
			)
			if err != nil {
				return err
			}

			if err := store.InsertCommandOutput(output); err != nil {
				return err
			}

			commandOutput = &output
		}

		if commandOutput != nil && sessionID != nil {
			event, ok := terraformTimelineEvent(
				args[0],
				recordCwd,
				commandOutput.Stdout,
				commandOutput.Stderr,
				*sessionID,
				event.EndedAt,
			)
			if ok {
				if _, err := store.InsertTimelineEvent(event); err != nil {
					return err
				}
			}
		}

		gitContext, err := gitcollector.Detect(recordCwd)
		if err != nil {
			return err
		}

		// A pre-command snapshot is supplied only for eligible Git mutations.
		// Comparing the two snapshots avoids attributing unrelated repository
		// changes merely because they occurred near a shell command.
		if sessionID != nil && recordGitBeforeFile != "" && gitcollector.MutatingCommand(args[0]) && recordExitCode == 0 {
			data, readErr := os.ReadFile(recordGitBeforeFile)
			if readErr == nil {
				var before gitcollector.Context
				if json.Unmarshal(data, &before) == nil {
					for _, change := range gitcollector.Compare(args[0], before, gitContext) {
						_, insertErr := store.InsertTimelineEvent(storage.TimelineEvent{
							SessionID: sessionID, EventType: change.Kind,
							Source: "git", ResourceType: "repository",
							Resource: redact.String(change.Resource), Summary: redact.String(change.Summary),
							OccurredAt: event.EndedAt,
						})
						if insertErr != nil {
							return insertErr
						}
					}
				}
			}
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
		Stdout:          redact.String(stdout),
		Stderr:          redact.String(stderr),
		StdoutBytes:     stdoutBytes,
		StderrBytes:     stderrBytes,
		StdoutTruncated: stdoutTruncated,
		StderrTruncated: stderrTruncated,
	}

	return output, nil
}

func terraformTimelineEvent(
	command string,
	cwd string,
	stdout string,
	stderr string,
	sessionID int64,
	occurredAt time.Time,
) (storage.TimelineEvent, bool) {
	fields := strings.Fields(command)

	if len(fields) < 2 {
		return storage.TimelineEvent{}, false
	}

	name := filepath.Base(fields[0])

	if name != "terraform" && name != "tofu" {
		return storage.TimelineEvent{}, false
	}

	subcommand := fields[1]

	output := stdout
	if stderr != "" {
		output += "\n" + stderr
	}

	var (
		summary   terraformcollector.ChangeSummary
		err       error
		eventType string
		label     string
	)

	switch subcommand {
	case "plan":
		summary, err = terraformcollector.ParsePlan(output)
		eventType = "plan"
		label = "Proposed"

	case "apply":
		summary, err = terraformcollector.ParseApply(output)
		eventType = "apply"
		label = "Changes"

	case "destroy":
		summary, err = terraformcollector.ParseDestroy(output)
		eventType = "destroy"
		label = "Changes"

	default:
		return storage.TimelineEvent{}, false
	}

	if err != nil {
		return storage.TimelineEvent{}, false
	}

	resource := filepath.Base(
		filepath.Clean(cwd),
	)

	return storage.TimelineEvent{
		SessionID:    &sessionID,
		EventType:    eventType,
		Source:       "terraform",
		ResourceType: "deployment",
		Resource:     resource,
		Summary: fmt.Sprintf(
			"%s: %s",
			label,
			terraformcollector.FormatSummary(summary),
		),
		OccurredAt: occurredAt,
	}, true
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

	recordCmd.Flags().StringVar(&recordGitBeforeFile, "git-before-file", "", "Pre-command Git metadata snapshot")
	recordCmd.Flags().StringVar(
		&recordStderrFile,
		"stderr-file",
		"",
		"path to captured stderr",
	)
}
