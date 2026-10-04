package cmd

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/it-odyssey/waketrail/internal/storage"
	"github.com/it-odyssey/waketrail/internal/ui"
	"github.com/spf13/cobra"
)

type displayEvent struct {
	OccurredAt time.Time
	Kind       string

	CommandEvent  *storage.CommandEvent
	TimelineEvent *storage.TimelineEvent
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

		sort.Slice(events, func(i, j int) bool {
			return events[i].OccurredAt.Before(
				events[j].OccurredAt,
			)
		})

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

func printReport(
	store *storage.Store,
	session storage.SessionRecord,
	events []displayEvent,
	commandEvents []storage.CommandEvent,
	timelineEvents []storage.TimelineEvent,
) {
	renderer := ui.Renderer()

	titleStyle := renderer.NewStyle().
		Bold(true).
		Foreground(ui.Accent)

	valueStyle := renderer.NewStyle().
		Foreground(ui.Accent)

	summaryBorder := lipgloss.RoundedBorder()

	summaryStyle := renderer.NewStyle().
		Border(summaryBorder).
		BorderForeground(ui.Muted).
		Padding(0, 2).
		Width(76)

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

	timeRange := session.StartedAt.Format("15:04:05")

	if session.EndedAt != nil {
		timeRange += " → " + session.EndedAt.Format("15:04:05")
	}

	failureLabel := pluralize(failureCount, "FAILURE", "FAILURES")
	recoveryLabel := pluralize(recoveryCount, "RECOVERY", "RECOVERIES")
	noteLabel := pluralize(noteCount, "NOTE", "NOTES")

	header := strings.Join([]string{
		titleStyle.Render("WAKE TRAIL"),
		valueStyle.Render(session.Name),
		"",
		fmt.Sprintf(
			"%s    %s    %s    %s",
			metric(renderer, duration, "duration"),
			metric(
				renderer,
				fmt.Sprintf("%d", len(events)),
				"events",
			),
			metric(
				renderer,
				fmt.Sprintf("%d", failureCount),
				strings.ToLower(failureLabel),
			),
			metric(
				renderer,
				fmt.Sprintf("%d", recoveryCount),
				strings.ToLower(recoveryLabel),
			),
		),
		renderer.NewStyle().
			Foreground(ui.Muted).
			Render(timeRange),
	}, "\n")

	if noteCount > 0 {
		header += "\n" +
			renderer.NewStyle().
				Foreground(ui.Note).
				Render(
					fmt.Sprintf(
						"%d %s",
						noteCount,
						noteLabel,
					),
				)
	}

	fmt.Println(summaryStyle.Render(header))
	fmt.Println()

	if len(events) == 0 {
		fmt.Println(
			renderer.NewStyle().
				Foreground(ui.Muted).
				Render(
					"No events recorded for this session.",
				),
		)

		return
	}

	for i, event := range events {
		switch event.Kind {
		case "command":
			printPrettyCommand(
				renderer,
				store,
				*event.CommandEvent,
			)

		case "timeline":
			printPrettyTimelineEvent(
				renderer,
				*event.TimelineEvent,
			)
		}

		if i < len(events)-1 {
			fmt.Println(
				renderer.NewStyle().
					Foreground(ui.Muted).
					Render("  │"),
			)
		}
	}
}

func pluralize(
	count int,
	singular string,
	plural string,
) string {
	if count == 1 {
		return singular
	}

	return plural
}

func metric(
	renderer *lipgloss.Renderer,
	value string,
	label string,
) string {
	valueStyle := renderer.NewStyle().
		Bold(true).
		Foreground(ui.Accent)

	labelStyle := renderer.NewStyle().
		Foreground(ui.Muted)

	return valueStyle.Render(value) +
		" " +
		labelStyle.Render(label)
}

func printPrettyCommand(
	renderer *lipgloss.Renderer,
	store *storage.Store,
	event storage.CommandEvent,
) {
	timeStyle := renderer.NewStyle().
		Foreground(ui.Muted)

	labelStyle := renderer.NewStyle().
		Bold(true).
		Foreground(ui.Command)

	commandStyle := renderer.NewStyle().
		Bold(true).
		Foreground(ui.Accent)

	metaStyle := renderer.NewStyle().
		Foreground(ui.Muted)

	success := event.ExitCode == 0

	status := renderer.NewStyle().
		Foreground(ui.Recovery).
		Render("✓")

	if !success {
		status = renderer.NewStyle().
			Foreground(ui.Failure).
			Render("✗")
	}

	fmt.Printf(
		"  %s  %s %s\n",
		timeStyle.Render(
			event.StartedAt.Format("15:04:05"),
		),
		status,
		labelStyle.Render("COMMAND"),
	)

	fmt.Printf(
		"  │           %s\n",
		commandStyle.Render(event.Command),
	)

	duration := event.EndedAt.
		Sub(event.StartedAt).
		Round(time.Millisecond)

	meta := fmt.Sprintf(
		"exit %d · %s",
		event.ExitCode,
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

		meta += fmt.Sprintf(
			" · %s@%s · %s",
			gitContext.Branch,
			commit,
			gitState,
		)
	}

	fmt.Printf(
		"  │           %s\n",
		metaStyle.Render(meta),
	)

	commandOutput, err := store.CommandOutputForCommandEvent(event.ID)

	if err != nil &&
		!errors.Is(
			err,
			storage.ErrCommandOutputNotFound,
		) {
		fmt.Printf(
			"  │           %s\n",
			metaStyle.Render("output unavailable"),
		)
	}

	if err == nil {
		if commandOutput.Stdout != "" {
			printOutputPreview(
				renderer,
				"output",
				commandOutput.Stdout,
			)
		}

		if commandOutput.Stderr != "" {
			printOutputPreview(
				renderer,
				"stderr",
				commandOutput.Stderr,
			)
		}
	}

	if showVerbose {
		fmt.Printf(
			"  │           %s\n",
			metaStyle.Render(
				"cwd "+ui.ShortPath(event.Cwd),
			),
		)
	}
}

func printOutputPreview(
	renderer *lipgloss.Renderer,
	label string,
	value string,
) {
	const maxLines = 3
	const maxChars = 240

	value = strings.TrimSpace(value)

	if value == "" {
		return
	}

	lines := strings.Split(value, "\n")

	truncated := false

	if len(lines) > maxLines {
		lines = lines[:maxLines]
		truncated = true
	}

	preview := strings.Join(lines, "\n")

	if len([]rune(preview)) > maxChars {
		runes := []rune(preview)
		preview = string(runes[:maxChars])
		truncated = true
	}

	labelStyle := renderer.NewStyle().
		Foreground(ui.Muted)

	outputStyle := renderer.NewStyle().
		Foreground(ui.Accent)

	fmt.Printf(
		"  │           %s\n",
		labelStyle.Render(label),
	)

	for _, line := range strings.Split(preview, "\n") {
		fmt.Printf(
			"  │             %s\n",
			outputStyle.Render(line),
		)
	}

	if truncated {
		fmt.Printf(
			"  │             %s\n",
			labelStyle.Render("…"),
		)
	}
}

func printPrettyTimelineEvent(
	renderer *lipgloss.Renderer,
	event storage.TimelineEvent,
) {
	switch event.EventType {
	case "failure":
		printEventCard(
			renderer,
			event,
			"FAILURE",
			ui.Failure,
			"✗",
		)

	case "recovery":
		printEventCard(
			renderer,
			event,
			"RECOVERY",
			ui.Recovery,
			"✓",
		)

	case "stopped":
		printEventCard(
			renderer,
			event,
			"STOPPED",
			ui.Muted,
			"■",
		)

	case "started":
		printEventCard(
			renderer,
			event,
			"STARTED",
			ui.State,
			"▶",
		)

	case "note":
		printEventCard(
			renderer,
			event,
			"NOTE",
			ui.Note,
			"✎",
		)

	case "state_change":
		printEventCard(
			renderer,
			event,
			"STATE CHANGE",
			ui.State,
			"↻",
		)

	default:
		printEventCard(
			renderer,
			event,
			strings.ToUpper(event.EventType),
			ui.Accent,
			"•",
		)
	}
}

func printEventCard(
	renderer *lipgloss.Renderer,
	event storage.TimelineEvent,
	label string,
	color lipgloss.TerminalColor,
	symbol string,
) {
	timeStyle := renderer.NewStyle().
		Foreground(ui.Muted)

	labelStyle := renderer.NewStyle().
		Bold(true).
		Foreground(color)

	sourceStyle := renderer.NewStyle().
		Foreground(ui.Muted)

	bodyStyle := renderer.NewStyle().
		Foreground(ui.Accent)

	border := lipgloss.RoundedBorder()

	cardStyle := renderer.NewStyle().
		Border(border).
		BorderForeground(color).
		Padding(0, 1).
		Width(62)

	body := formatEventBody(
		renderer,
		event,
		bodyStyle,
		sourceStyle,
	)

	card := cardStyle.Render(body)

	lines := strings.Split(card, "\n")

	for i, line := range lines {
		if i == 0 {
			fmt.Printf(
				"  %s  %s %s\n",
				timeStyle.Render(
					event.OccurredAt.Format("15:04:05"),
				),
				renderer.NewStyle().
					Foreground(color).
					Render(symbol),
				labelStyle.Render(label),
			)
		}

		fmt.Printf(
			"  │           %s\n",
			line,
		)
	}
}

func formatEventBody(
	renderer *lipgloss.Renderer,
	event storage.TimelineEvent,
	bodyStyle lipgloss.Style,
	sourceStyle lipgloss.Style,
) string {
	source := sourceStyle.Render(event.Source)

	if event.EventType == "note" {
		return bodyStyle.Render(event.Summary)
	}

	parts := strings.SplitN(
		event.Summary,
		": ",
		2,
	)

	if len(parts) != 2 {
		return source + "\n\n" +
			bodyStyle.Render(
				formatTransitionArrow(
					event.Summary,
				),
			)
	}

	name := renderer.NewStyle().
		Bold(true).
		Foreground(ui.Accent).
		Render(parts[0])

	return source +
		" · " +
		name +
		"\n\n" +
		bodyStyle.Render(
			formatTransitionArrow(parts[1]),
		)
}

func formatTransitionArrow(
	value string,
) string {
	return strings.ReplaceAll(
		value,
		" -> ",
		" → ",
	)
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
