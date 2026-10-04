package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/it-odyssey/waketrail/internal/storage"
	"github.com/spf13/cobra"
)

var exportOutput string

var exportCmd = &cobra.Command{
	Use:   "export [session-name]",
	Short: "Export a WakeTrail session as Markdown",
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

		commandEvents, err := store.CommandEventsForSession(
			session.ID,
		)
		if err != nil {
			return err
		}

		timelineEvents, err := store.TimelineEventsForSession(
			session.ID,
		)
		if err != nil {
			return err
		}

		events := buildExportEvents(
			commandEvents,
			timelineEvents,
		)

		content, err := buildMarkdownExport(
			store,
			session,
			events,
			timelineEvents,
		)
		if err != nil {
			return err
		}

		outputPath := exportOutput

		if outputPath == "" {
			outputPath = safeExportFilename(
				session.Name,
			) + ".md"
		}

		if err := os.WriteFile(
			outputPath,
			[]byte(content),
			0644,
		); err != nil {
			return err
		}

		absolutePath, err := filepath.Abs(outputPath)
		if err != nil {
			absolutePath = outputPath
		}

		fmt.Printf(
			"Exported WakeTrail session to %s\n",
			absolutePath,
		)

		return nil
	},
}

func buildExportEvents(
	commandEvents []storage.CommandEvent,
	timelineEvents []storage.TimelineEvent,
) []displayEvent {
	events := make(
		[]displayEvent,
		0,
		len(commandEvents)+len(timelineEvents),
	)

	for i := range commandEvents {
		event := &commandEvents[i]

		events = append(events, displayEvent{
			OccurredAt:   event.StartedAt,
			Kind:         "command",
			CommandEvent: event,
		})
	}

	for i := range timelineEvents {
		event := &timelineEvents[i]

		events = append(events, displayEvent{
			OccurredAt:    event.OccurredAt,
			Kind:          "timeline",
			TimelineEvent: event,
		})
	}

	sortDisplayEvents(events)

	return correlateDockerLifecycleEvents(events)
}

func sortDisplayEvents(events []displayEvent) {
	for i := 1; i < len(events); i++ {
		for j := i; j > 0 &&
			events[j].OccurredAt.Before(
				events[j-1].OccurredAt,
			); j-- {
			events[j], events[j-1] =
				events[j-1], events[j]
		}
	}
}

func buildMarkdownExport(
	store *storage.Store,
	session storage.SessionRecord,
	events []displayEvent,
	timelineEvents []storage.TimelineEvent,
) (string, error) {
	var builder strings.Builder

	failureCount := 0
	recoveryCount := 0
	noteCount := 0

	for _, event := range timelineEvents {
		switch event.EventType {
		case "failure":
			failureCount++

		case "recovery":
			recoveryCount++

		case "note":
			noteCount++
		}
	}

	duration := "recording"

	if session.EndedAt != nil {
		duration = session.EndedAt.
			Sub(session.StartedAt).
			Round(time.Second).
			String()
	}

	builder.WriteString("# WakeTrail Session: ")
	builder.WriteString(session.Name)
	builder.WriteString("\n\n")

	builder.WriteString("## Summary\n\n")

	fmt.Fprintf(
		&builder,
		"- Started: %s\n",
		session.StartedAt.Format(
			"2006-01-02 15:04:05",
		),
	)

	if session.EndedAt != nil {
		fmt.Fprintf(
			&builder,
			"- Ended: %s\n",
			session.EndedAt.Format(
				"2006-01-02 15:04:05",
			),
		)
	}

	fmt.Fprintf(
		&builder,
		"- Duration: %s\n",
		duration,
	)

	fmt.Fprintf(
		&builder,
		"- Events: %d\n",
		len(events),
	)

	fmt.Fprintf(
		&builder,
		"- Failures: %d\n",
		failureCount,
	)

	fmt.Fprintf(
		&builder,
		"- Recoveries: %d\n",
		recoveryCount,
	)

	if noteCount > 0 {
		fmt.Fprintf(
			&builder,
			"- Notes: %d\n",
			noteCount,
		)
	}

	builder.WriteString("\n## Timeline\n\n")

	for _, event := range events {
		switch event.Kind {
		case "command":
			if err := writeMarkdownCommand(
				&builder,
				store,
				*event.CommandEvent,
			); err != nil {
				return "", err
			}

		case "timeline":
			writeMarkdownTimelineEvent(
				&builder,
				*event.TimelineEvent,
				event.CorrelatedCommand,
			)
		}
	}

	return builder.String(), nil
}

func writeMarkdownCommand(
	builder *strings.Builder,
	store *storage.Store,
	event storage.CommandEvent,
) error {
	duration := event.EndedAt.
		Sub(event.StartedAt).
		Round(time.Millisecond)

	fmt.Fprintf(
		builder,
		"### %s — COMMAND\n\n",
		event.StartedAt.Format("15:04:05"),
	)

	builder.WriteString("```text\n")
	builder.WriteString(event.Command)
	builder.WriteString("\n```\n\n")

	fmt.Fprintf(
		builder,
		"- Exit code: %d\n",
		event.ExitCode,
	)

	fmt.Fprintf(
		builder,
		"- Duration: %s\n",
		duration,
	)

	gitContext, err := store.GitContextForCommandEvent(
		event.ID,
	)

	if err == nil {
		commit := gitContext.CommitSHA

		if len(commit) > 7 {
			commit = commit[:7]
		}

		gitState := "clean"

		if gitContext.Dirty {
			gitState = "dirty"
		}

		fmt.Fprintf(
			builder,
			"- Git: %s@%s (%s)\n",
			gitContext.Branch,
			commit,
			gitState,
		)
	}

	output, err := store.CommandOutputForCommandEvent(
		event.ID,
	)

	if err != nil &&
		!errors.Is(
			err,
			storage.ErrCommandOutputNotFound,
		) {
		return err
	}

	if err == nil {
		if output.Stdout != "" {
			builder.WriteString(
				"\n**stdout**\n\n```text\n",
			)

			builder.WriteString(
				strings.TrimSpace(output.Stdout),
			)

			builder.WriteString("\n```\n")
		}

		if output.Stderr != "" {
			builder.WriteString(
				"\n**stderr**\n\n```text\n",
			)

			builder.WriteString(
				strings.TrimSpace(output.Stderr),
			)

			builder.WriteString("\n```\n")
		}

		if output.StdoutTruncated ||
			output.StderrTruncated {
			builder.WriteString(
				"\n> Output was truncated during capture.\n",
			)
		}
	}

	builder.WriteString("\n")

	return nil
}

func writeMarkdownTimelineEvent(
	builder *strings.Builder,
	event storage.TimelineEvent,
	command *storage.CommandEvent,
) {
	label := strings.ToUpper(
		strings.ReplaceAll(
			event.EventType,
			"_",
			" ",
		),
	)

	fmt.Fprintf(
		builder,
		"### %s — %s\n\n",
		event.OccurredAt.Format("15:04:05"),
		label,
	)

	fmt.Fprintf(
		builder,
		"- Source: %s\n",
		event.Source,
	)

	fmt.Fprintf(
		builder,
		"- Event: %s\n",
		formatTransitionArrow(event.Summary),
	)

	if command != nil {
		fmt.Fprintf(
			builder,
			"- Command: `%s`\n",
			command.Command,
		)
	}

	builder.WriteString("\n")
}

func safeExportFilename(name string) string {
	name = strings.TrimSpace(name)

	if name == "" {
		return "waketrail-export"
	}

	var builder strings.Builder

	for _, char := range name {
		switch {
		case unicode.IsLetter(char),
			unicode.IsDigit(char),
			char == '-',
			char == '_',
			char == '.':
			builder.WriteRune(char)

		default:
			builder.WriteRune('-')
		}
	}

	result := strings.Trim(
		builder.String(),
		"-.",
	)

	if result == "" {
		return "waketrail-export"
	}

	return result
}

func init() {
	exportCmd.Flags().StringVarP(
		&exportOutput,
		"output",
		"o",
		"",
		"output Markdown file",
	)

	rootCmd.AddCommand(exportCmd)
}
