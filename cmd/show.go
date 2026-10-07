package cmd

import (
	"errors"
	"fmt"
	"path/filepath"
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

		events = correlateLifecycleEvents(events)

		if !showVerbose {
			events = correlateTerraformEvents(events)
			events = groupKubernetesActivities(events)
			events = filterWakeTrailCommands(events)
		}

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

func correlateLifecycleEvents(
	events []displayEvent,
) []displayEvent {
	result := make(
		[]displayEvent,
		0,
		len(events),
	)

	for _, event := range events {
		if !isLifecycleEvent(event) ||
			len(result) == 0 {
			result = append(result, event)
			continue
		}

		previous := result[len(result)-1]

		if previous.Kind != "command" ||
			previous.CommandEvent == nil {
			result = append(result, event)
			continue
		}

		command := previous.CommandEvent

		if event.OccurredAt.Before(command.EndedAt) ||
			event.OccurredAt.Sub(command.EndedAt) > 5*time.Second {
			result = append(result, event)
			continue
		}

		if !commandMatchesLifecycle(
			*command,
			*event.TimelineEvent,
		) {
			result = append(result, event)
			continue
		}

		event.CorrelatedCommand = command

		// Replace the standalone command with the correlated
		// Docker lifecycle event in the display timeline.
		result[len(result)-1] = event
	}

	return result
}

func correlateTerraformEvents(
	events []displayEvent,
) []displayEvent {
	result := make(
		[]displayEvent,
		0,
		len(events),
	)

	for _, event := range events {
		if event.Kind != "timeline" ||
			event.TimelineEvent == nil ||
			event.TimelineEvent.Source != "terraform" ||
			len(result) == 0 {
			result = append(result, event)
			continue
		}

		previous := result[len(result)-1]

		if previous.Kind != "command" ||
			previous.CommandEvent == nil {
			result = append(result, event)
			continue
		}

		command := previous.CommandEvent

		if event.OccurredAt.Before(
			command.EndedAt,
		) ||
			event.OccurredAt.Sub(
				command.EndedAt,
			) > 5*time.Second {
			result = append(result, event)
			continue
		}

		if !commandMatchesTerraform(
			*command,
		) {
			result = append(result, event)
			continue
		}

		event.CorrelatedCommand = command

		// Replace the standalone Terraform command with its
		// semantic timeline event in the normal presentation.
		result[len(result)-1] = event
	}

	return result
}

func commandMatchesTerraform(
	command storage.CommandEvent,
) bool {
	fields := strings.Fields(
		command.Command,
	)

	fields = stripCommandPrefixes(fields)

	if len(fields) < 2 {
		return false
	}

	name := filepath.Base(fields[0])

	if name != "terraform" &&
		name != "tofu" {
		return false
	}

	switch fields[1] {
	case "plan",
		"apply",
		"destroy":
		return true

	default:
		return false
	}
}

func isLifecycleEvent(
	event displayEvent,
) bool {
	if event.Kind != "timeline" ||
		event.TimelineEvent == nil {
		return false
	}

	switch event.TimelineEvent.EventType {
	case "created",
		"removed",
		"started",
		"stopped":
		return true

	default:
		return false
	}
}

func commandMatchesLifecycle(
	command storage.CommandEvent,
	event storage.TimelineEvent,
) bool {
	switch event.Source {
	case "docker":
		return dockerCommandMatchesLifecycle(
			command,
			event,
		)

	case "systemd":
		return systemdCommandMatchesLifecycle(
			command,
			event,
		)

	default:
		return false
	}
}

func dockerCommandMatchesLifecycle(
	command storage.CommandEvent,
	event storage.TimelineEvent,
) bool {
	containerName := timelineResourceName(event)

	if containerName == "" {
		return false
	}

	fields := strings.Fields(command.Command)
	fields = stripCommandPrefixes(fields)

	if len(fields) < 2 ||
		filepath.Base(fields[0]) != "docker" {
		return false
	}

	switch event.EventType {
	case "created":
		return dockerCommandCreatesContainer(
			fields,
			containerName,
		)

	case "removed":
		return dockerCommandRemovesContainer(
			fields,
			containerName,
		)

	case "stopped":
		return len(fields) >= 3 &&
			fields[1] == "stop" &&
			fields[len(fields)-1] == containerName

	case "started":
		return len(fields) >= 3 &&
			fields[1] == "start" &&
			fields[len(fields)-1] == containerName

	default:
		return false
	}
}

func dockerCommandCreatesContainer(
	fields []string,
	containerName string,
) bool {
	if len(fields) < 2 {
		return false
	}

	switch fields[1] {
	case "run", "create":
	default:
		return false
	}

	for i := 2; i < len(fields); i++ {
		if fields[i] == "--name" &&
			i+1 < len(fields) {
			return fields[i+1] == containerName
		}

		if strings.HasPrefix(
			fields[i],
			"--name=",
		) {
			return strings.TrimPrefix(
				fields[i],
				"--name=",
			) == containerName
		}
	}

	return false
}

func dockerCommandRemovesContainer(
	fields []string,
	containerName string,
) bool {
	if len(fields) < 3 {
		return false
	}

	if fields[1] == "rm" {
		return fields[len(fields)-1] ==
			containerName
	}

	if len(fields) >= 4 &&
		fields[1] == "container" &&
		fields[2] == "rm" {
		return fields[len(fields)-1] ==
			containerName
	}

	return false
}

func systemdCommandMatchesLifecycle(
	command storage.CommandEvent,
	event storage.TimelineEvent,
) bool {
	serviceName := timelineResourceName(event)

	if serviceName == "" {
		return false
	}

	fields := strings.Fields(command.Command)

	fields = stripCommandPrefixes(fields)

	if len(fields) < 3 ||
		filepath.Base(fields[0]) != "systemctl" {
		return false
	}

	expectedAction := ""

	switch event.EventType {
	case "stopped":
		expectedAction = "stop"

	case "started":
		expectedAction = "start"

	default:
		return false
	}

	if fields[1] != expectedAction {
		return false
	}

	return fields[len(fields)-1] == serviceName
}

func stripCommandPrefixes(
	fields []string,
) []string {
	for len(fields) > 0 {
		switch fields[0] {
		case "sudo", "command":
			fields = fields[1:]

			for len(fields) > 0 &&
				strings.HasPrefix(fields[0], "-") {
				fields = fields[1:]
			}

		default:
			return fields
		}
	}

	return fields
}

func filterWakeTrailCommands(
	events []displayEvent,
) []displayEvent {
	filtered := make(
		[]displayEvent,
		0,
		len(events),
	)

	for _, event := range events {
		if event.Kind == "command" &&
			event.CommandEvent != nil &&
			(isWakeTrailCommand(
				event.CommandEvent.Command,
			) ||
				isShellHousekeepingCommand(
					event.CommandEvent.Command,
				)) {
			continue
		}

		filtered = append(filtered, event)
	}

	return filtered
}

func isWakeTrailCommand(command string) bool {
	fields := strings.Fields(command)

	if len(fields) == 0 {
		return false
	}

	for len(fields) > 0 &&
		(fields[0] == "sudo" ||
			fields[0] == "command") {
		fields = fields[1:]
	}

	if len(fields) == 0 {
		return false
	}

	return filepath.Base(fields[0]) == "waketrail"
}

func isShellHousekeepingCommand(
	command string,
) bool {
	fields := strings.Fields(command)

	if len(fields) < 2 {
		return false
	}

	if fields[0] != "." &&
		fields[0] != "source" {
		return false
	}

	path := strings.Trim(
		fields[1],
		`"'`,
	)

	return strings.Contains(
		path,
		"/.local/share/",
	) &&
		strings.HasSuffix(
			path,
			"/bin/env",
		)
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
				event.CorrelatedCommand,
			)

		case "activity":
			printPrettyActivity(
				renderer,
				*event.Activity,
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

func printPrettyActivity(
	renderer *lipgloss.Renderer,
	activity displayActivity,
) {
	color := ui.State
	symbol := "↻"

	switch activity.EventType {
	case "failure":
		color = ui.Failure
		symbol = "✗"

	case "recovery":
		color = ui.Recovery
		symbol = "✓"
	}

	timeStyle := renderer.NewStyle().
		Foreground(ui.Muted)

	headerStyle := renderer.NewStyle().
		Bold(true).
		Foreground(color)

	valueStyle := renderer.NewStyle().
		Foreground(ui.Accent)

	border := lipgloss.RoundedBorder()

	cardStyle := renderer.NewStyle().
		Border(border).
		BorderForeground(color).
		Padding(0, 1).
		Width(72)

	eventLabel := strings.ToUpper(
		strings.ReplaceAll(
			activity.EventType,
			"_",
			" ",
		),
	)

	resourceLabel := displayResourceType(
		activity.ResourceType,
	)

	var body strings.Builder

	fmt.Fprintf(
		&body,
		"%s: %s\n\n",
		resourceLabel,
		activity.Resource,
	)

	fmt.Fprintf(
		&body,
		"%s: Effects:\n",
		activity.OccurredAt.Format("15:04:05"),
	)

	for i, effect := range summarizeActivityEffects(
		activity.Effects,
	) {
		if i > 0 {
			body.WriteString("\n")
		}

		fmt.Fprintf(
			&body,
			"  %s: %s\n",
			displayResourceType(
				effect.ResourceType,
			),
			effect.Resource,
		)

		for _, line := range formatActivityEffectLines(
			effect,
		) {
			fmt.Fprintf(
				&body,
				"    %s\n",
				line,
			)
		}
	}

	if activity.Command != nil {
		fmt.Fprintf(
			&body,
			"\nCommand: %s",
			activity.Command.Command,
		)
	}

	card := cardStyle.Render(
		valueStyle.Render(
			strings.TrimSpace(
				body.String(),
			),
		),
	)

	fmt.Printf(
		"  %s  %s %s\n",
		timeStyle.Render(
			activity.OccurredAt.Format("15:04:05"),
		),
		renderer.NewStyle().
			Foreground(color).
			Render(symbol),
		headerStyle.Render(
			titleSource(activity.Source)+
				": "+
				eventLabel,
		),
	)

	for _, line := range strings.Split(
		card,
		"\n",
	) {
		fmt.Printf(
			"  │           %s\n",
			line,
		)
	}
}

func printPrettyTimelineEvent(
	renderer *lipgloss.Renderer,
	event storage.TimelineEvent,
	command *storage.CommandEvent,
) {
	if event.EventType == "note" {
		printEventCard(
			renderer,
			event,
			command,
			"NOTE",
			ui.Note,
			"✎",
		)

		return
	}

	label := strings.ToUpper(
		strings.ReplaceAll(
			event.EventType,
			"_",
			" ",
		),
	)

	color := ui.State
	symbol := "↻"

	switch event.EventType {
	case "failure":
		color = ui.Failure
		symbol = "✗"

	case "recovery":
		color = ui.Recovery
		symbol = "✓"

	case "stopped":
		color = ui.Muted
		symbol = "■"

	case "started":
		color = ui.State
		symbol = "▶"

	case "plan":
		color = ui.State
		symbol = "◇"

	case "apply":
		color = ui.Recovery
		symbol = "✓"

	case "destroy":
		color = ui.Muted
		symbol = "■"

	case "created":
		color = ui.State
		symbol = "+"

	case "removed":
		color = ui.Muted
		symbol = "−"
	}

	printCollectorCard(
		renderer,
		event.OccurredAt,
		event.Source,
		label,
		color,
		symbol,
		timelineCardFields(
			event,
			command,
		),
	)
}

func printEventCard(
	renderer *lipgloss.Renderer,
	event storage.TimelineEvent,
	command *storage.CommandEvent,
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
		command,
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

func timelineResourceName(
	event storage.TimelineEvent,
) string {
	if event.Resource != "" {
		return event.Resource
	}

	parts := strings.SplitN(
		event.Summary,
		": ",
		2,
	)

	if len(parts) == 2 {
		return parts[0]
	}

	return ""
}

func timelineDisplaySummary(
	event storage.TimelineEvent,
) string {
	if event.Resource != "" {
		prefix := event.Resource + ": "

		if strings.HasPrefix(
			event.Summary,
			prefix,
		) {
			return strings.TrimPrefix(
				event.Summary,
				prefix,
			)
		}

		return event.Summary
	}

	parts := strings.SplitN(
		event.Summary,
		": ",
		2,
	)

	if len(parts) == 2 {
		return parts[1]
	}

	return event.Summary
}

func formatEventBody(
	renderer *lipgloss.Renderer,
	event storage.TimelineEvent,
	command *storage.CommandEvent,
	bodyStyle lipgloss.Style,
	sourceStyle lipgloss.Style,
) string {
	if event.EventType == "note" {
		return bodyStyle.Render(event.Summary)
	}

	source := sourceStyle.Render(event.Source)

	resource := timelineResourceName(event)
	summary := timelineDisplaySummary(event)

	if event.Source == "terraform" && resource != "" {
		resourceLabel := displayResourceType(
			event.ResourceType,
		)

		name := renderer.NewStyle().
			Bold(true).
			Foreground(ui.Accent).
			Render(resource)

		body := sourceStyle.Render(
			resourceLabel+": ",
		) + name +
			"\n" +
			bodyStyle.Render(
				formatTransitionArrow(summary),
			)

		if command != nil {
			body += "\n" +
				sourceStyle.Render("command · ") +
				bodyStyle.Render(command.Command)
		}

		return body
	}

	var body string

	if resource == "" {
		body = source +
			"\n\n" +
			bodyStyle.Render(
				formatTransitionArrow(summary),
			)
	} else {
		name := renderer.NewStyle().
			Bold(true).
			Foreground(ui.Accent).
			Render(resource)

		body = source +
			" · " +
			name +
			"\n\n" +
			bodyStyle.Render(
				formatTransitionArrow(summary),
			)
	}

	if command != nil {
		body += "\n\n" +
			sourceStyle.Render("command · ") +
			bodyStyle.Render(command.Command)
	}

	return body
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
