package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/it-odyssey/waketrail/internal/storage"
	"github.com/it-odyssey/waketrail/internal/ui"
)

type cardField struct {
	Label string
	Value string
}

func printCollectorCard(
	renderer *lipgloss.Renderer,
	occurredAt time.Time,
	source string,
	eventLabel string,
	color lipgloss.TerminalColor,
	symbol string,
	fields []cardField,
) {
	timeStyle := renderer.NewStyle().
		Foreground(ui.Muted)

	headerStyle := renderer.NewStyle().
		Bold(true).
		Foreground(color)

	border := lipgloss.RoundedBorder()

	cardStyle := renderer.NewStyle().
		Border(border).
		BorderForeground(color).
		Padding(0, 1).
		Width(72)

	card := cardStyle.Render(
		formatCardFields(
			renderer,
			fields,
		),
	)

	header := titleSource(source) +
		": " +
		eventLabel

	fmt.Printf(
		"  %s  %s %s\n",
		timeStyle.Render(
			occurredAt.Format("15:04:05"),
		),
		renderer.NewStyle().
			Foreground(color).
			Render(symbol),
		headerStyle.Render(header),
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

func formatCardFields(
	renderer *lipgloss.Renderer,
	fields []cardField,
) string {
	if len(fields) == 0 {
		return ""
	}

	labelWidth := 0

	for _, field := range fields {
		if len(field.Label) > labelWidth {
			labelWidth = len(field.Label)
		}
	}

	labelStyle := renderer.NewStyle().
		Foreground(ui.Muted)

	valueStyle := renderer.NewStyle().
		Foreground(ui.Accent)

	var body strings.Builder

	for i, field := range fields {
		if i > 0 {
			body.WriteString("\n")
		}

		label := fmt.Sprintf(
			"%-*s",
			labelWidth,
			field.Label,
		)

		lines := strings.Split(
			field.Value,
			"\n",
		)

		body.WriteString(
			labelStyle.Render(label + " : "),
		)

		if len(lines) > 0 {
			body.WriteString(
				valueStyle.Render(lines[0]),
			)
		}

		for _, line := range lines[1:] {
			body.WriteString("\n")

			body.WriteString(
				strings.Repeat(
					" ",
					labelWidth+3,
				),
			)

			body.WriteString(
				valueStyle.Render(line),
			)
		}
	}

	return body.String()
}

func timelineCardFields(
	event storage.TimelineEvent,
	command *storage.CommandEvent,
) []cardField {
	resource := timelineResourceName(event)
	summary := timelineDisplaySummary(event)
	if event.Source == "docker" {
		resourcePrefix := resource + " "
		if strings.HasPrefix(summary, resourcePrefix) {
			summary = strings.TrimPrefix(summary, resourcePrefix)
		}
	}

	var fields []cardField

	if resource != "" {
		fields = append(
			fields,
			cardField{
				Label: displayResourceType(
					event.ResourceType,
				),
				Value: resource,
			},
		)
	}

	effectLabel := "Effect"
	effectValue := formatTransitionArrow(
		summary,
	)

	if event.Source == "terraform" {
		switch {
		case strings.HasPrefix(
			summary,
			"Proposed: ",
		):
			effectLabel = "Proposed"
			effectValue = strings.TrimPrefix(
				summary,
				"Proposed: ",
			)

		case strings.HasPrefix(
			summary,
			"Changes: ",
		):
			effectLabel = "Changes"
			effectValue = strings.TrimPrefix(
				summary,
				"Changes: ",
			)
		}
	}

	if effectValue != "" {
		fields = append(
			fields,
			cardField{
				Label: effectLabel,
				Value: effectValue,
			},
		)
	}

	if command != nil {
		fields = append(
			fields,
			cardField{
				Label: "Command",
				Value: command.Command,
			},
		)
	}

	return fields
}

// Notes intentionally use a distinct simple card rather than a collector header.
func printNoteCard(renderer *lipgloss.Renderer, event storage.TimelineEvent) {
	style := renderer.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ui.Note).Padding(0, 1).Width(62)
	fmt.Printf("  %s  %s %s\n", renderer.NewStyle().Foreground(ui.Muted).Render(event.OccurredAt.Format("15:04:05")),
		renderer.NewStyle().Foreground(ui.Note).Render("✎"), renderer.NewStyle().Foreground(ui.Note).Bold(true).Render("NOTE"))
	card := style.Render(renderer.NewStyle().Foreground(ui.Accent).Render(event.Summary))
	for _, line := range strings.Split(card, "\n") {
		fmt.Printf("  │           %s\n", line)
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

func formatTransitionArrow(
	value string,
) string {
	return strings.ReplaceAll(
		value,
		" -> ",
		" → ",
	)
}

func printPrettyTimelineEvent(
	renderer *lipgloss.Renderer,
	event storage.TimelineEvent,
	command *storage.CommandEvent,
) {
	if event.EventType == "note" {
		printNoteCard(renderer, event)
		return
	}

	kind := normalizedTimelineEventType(event)
	label := strings.ToUpper(strings.ReplaceAll(kind, "_", " "))

	color := ui.State
	symbol := "↻"

	switch kind {
	case "failure":
		color = ui.Failure
		symbol = "✗"

	case "recovery":
		color = ui.Recovery
		symbol = "✓"

	case "created":
		color = ui.State
		symbol = "+"

	case "removed":
		color = ui.Muted
		symbol = "−"

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

// normalizedTimelineEventType improves the presentation of older SQLite rows
// without mutating the forensic event stored during recording.
func normalizedTimelineEventType(event storage.TimelineEvent) string {
	if event.Source == "docker" && event.EventType == "state_change" {
		summary := timelineDisplaySummary(event)
		if strings.Contains(summary, "appeared:") {
			return "created"
		}
		if strings.HasSuffix(summary, "disappeared") {
			return "removed"
		}
	}
	// A newly started pod may be Running but not Ready yet; it has not
	// necessarily failed. Preserve the original classification in storage.
	if event.Source == "kubernetes" && event.ResourceType == "pod" &&
		event.EventType == "failure" &&
		strings.Contains(event.Summary, " → Running ready 0/") &&
		!strings.Contains(event.Summary, "CrashLoopBackOff") &&
		!strings.Contains(event.Summary, "ImagePullBackOff") {
		return "state_change"
	}
	return event.EventType
}

func printPrettyActivity(renderer *lipgloss.Renderer, activity displayActivity) {
	color, symbol := ui.State, "↻"
	switch activity.EventType {
	case "failure":
		color, symbol = ui.Failure, "✗"
	case "recovery":
		color, symbol = ui.Recovery, "✓"
	case "apply":
		color, symbol = ui.Recovery, "✓"
	}
	fields := []cardField{{Label: displayResourceType(activity.ResourceType), Value: activity.Resource}}
	var effects strings.Builder
	for i, effect := range summarizeActivityEffects(activity.Effects) {
		if i > 0 {
			effects.WriteString("\n")
		}
		fmt.Fprintf(&effects, "%s: %s\n", displayResourceType(effect.ResourceType), effect.Resource)
		for _, line := range formatActivityEffectLines(effect) {
			fmt.Fprintf(&effects, "  %s\n", line)
		}
	}
	fields = append(fields, cardField{Label: "Effects", Value: strings.TrimSuffix(effects.String(), "\n")})
	if activity.Command != nil {
		fields = append(fields, cardField{Label: "Command", Value: activity.Command.Command})
	}
	printCollectorCard(renderer, activity.OccurredAt, activity.Source,
		strings.ToUpper(strings.ReplaceAll(activity.EventType, "_", " ")), color, symbol, fields)
}
