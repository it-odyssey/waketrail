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
