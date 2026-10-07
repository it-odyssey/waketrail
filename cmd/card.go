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

// The card width is fixed in the current terminal layout. Each field is
// constrained before styling, so ANSI color sequences cannot corrupt widths.
const collectorCardWidth = 72
const collectorCardContentWidth = collectorCardWidth - 4 // border and padding

func formatCardFields(renderer *lipgloss.Renderer, fields []cardField) string {
	if len(fields) == 0 {
		return ""
	}
	labelWidth := 0
	for _, field := range fields {
		if len(field.Label) > labelWidth {
			labelWidth = len(field.Label)
		}
	}
	labelStyle := renderer.NewStyle().Foreground(ui.Muted)
	valueStyle := renderer.NewStyle().Foreground(ui.Accent)
	var body strings.Builder
	for i, field := range fields {
		if i != 0 {
			body.WriteByte('\n')
		}
		label := fmt.Sprintf("%-*s : ", labelWidth, field.Label)
		width := collectorCardContentWidth - lipgloss.Width(label)
		if width < 12 {
			width = 12
		}
		lines := strings.Split(field.Value, "\n")
		for j, line := range lines {
			if j == 0 {
				body.WriteString(labelStyle.Render(label))
			} else {
				body.WriteByte('\n')
				body.WriteString(strings.Repeat(" ", lipgloss.Width(label)))
			}
			// Structured multi-line values must never wrap outside their
			// allocated width; the raw value remains in SQLite and verbose.
			if field.Label == "Effects" || field.Label == "Resource" || field.Label == "Deployment" || field.Label == "Pod" || field.Label == "Container" || field.Label == "Service" {
				line = ellipsizeMiddle(line, width)
				body.WriteString(valueStyle.Render(line))
				continue
			}
			wrapped := wrapFieldLine(line, width)
			for k, part := range wrapped {
				if k > 0 {
					body.WriteByte('\n')
					body.WriteString(strings.Repeat(" ", lipgloss.Width(label)))
				}
				body.WriteString(valueStyle.Render(part))
			}
		}
	}
	return body.String()
}

// Middle ellipsis keeps unique suffixes of Kubernetes-generated names.
func ellipsizeMiddle(text string, width int) string {
	if width < 2 {
		return "…"
	}
	if lipgloss.Width(text) <= width {
		return text
	}
	runes := []rune(text)
	left, right := "", ""
	for len(runes) > 0 && lipgloss.Width(left)+lipgloss.Width(right)+1 < width {
		if len(runes) == 0 {
			break
		}
		c := string(runes[0])
		runes = runes[1:]
		if lipgloss.Width(left)+lipgloss.Width(right)+lipgloss.Width(c)+1 > width {
			break
		}
		left += c
		if len(runes) == 0 {
			break
		}
		c = string(runes[len(runes)-1])
		runes = runes[:len(runes)-1]
		if lipgloss.Width(left)+lipgloss.Width(right)+lipgloss.Width(c)+1 > width {
			break
		}
		right = c + right
	}
	return left + "…" + right
}

// Commands wrap at whitespace where possible; no information is discarded.
func wrapFieldLine(text string, width int) []string {
	if lipgloss.Width(text) <= width {
		return []string{text}
	}
	var result []string
	remaining := text
	for lipgloss.Width(remaining) > width {
		chars := []rune(remaining)
		cut, size, lastSpace := 0, 0, -1
		for i, ch := range chars {
			w := lipgloss.Width(string(ch))
			if size+w > width {
				break
			}
			size += w
			cut = i + 1
			if ch == ' ' {
				lastSpace = i
			}
		}
		if cut == 0 {
			cut = 1
		}
		if lastSpace > 0 && lastSpace >= cut/2 {
			cut = lastSpace + 1
		}
		result = append(result, strings.TrimRight(string(chars[:cut]), " "))
		remaining = strings.TrimLeft(string(chars[cut:]), " ")
	}
	if remaining != "" {
		result = append(result, remaining)
	}
	return result
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
	style := renderer.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ui.Note).Padding(0, 1).Width(collectorCardWidth)
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
	if event.Source != "kubernetes" || (event.EventType != "failure" && event.EventType != "recovery") {
		return event.EventType
	}
	summary := formatTransitionArrow(timelineDisplaySummary(event))
	previous := strings.SplitN(summary, " → ", 2)[0]
	// Only definitive health signals can establish an incident. A controller
	// with fewer available replicas during a rollout is not itself failed.
	switch event.ResourceType {
	case "pod":
		if event.EventType == "failure" && !kubernetesPodError(strings.TrimPrefix(summary, previous+" → ")) {
			return "state_change"
		}
		if event.EventType == "recovery" && !kubernetesPodError(previous) {
			return "state_change"
		}
	case "deployment", "statefulset":
		return "state_change"
	case "daemonset":
		// Misscheduled Pods are definitive; ordinary convergence is not.
		if event.EventType == "failure" && !hasMisscheduled(strings.TrimPrefix(summary, previous+" → ")) {
			return "state_change"
		}
		if event.EventType == "recovery" && !hasMisscheduled(previous) {
			return "state_change"
		}
	}
	return event.EventType
}

func kubernetesPodError(summary string) bool {
	for _, marker := range []string{"CrashLoopBackOff", "ImagePullBackOff", "ErrImagePull", "CreateContainerConfigError", "CreateContainerError", "OOMKilled", "Failed", "Unknown", "/Error"} {
		if strings.Contains(summary, marker) {
			return true
		}
	}
	return false
}

func hasMisscheduled(summary string) bool {
	// Counts only explicit nonzero misscheduling, never absence of the field.
	for _, segment := range strings.Split(summary, " → ") {
		fields := strings.Fields(segment)
		for i := 0; i+1 < len(fields); i++ {
			if fields[i] == "misscheduled" && fields[i+1] != "0" {
				return true
			}
		}
	}
	return false
}

func printPrettyActivity(renderer *lipgloss.Renderer, activity displayActivity) {
	color, symbol := ui.State, "↻"
	switch activity.EventType {
	case "failure":
		color, symbol = ui.Failure, "✗"
	case "recovery":
		color, symbol = ui.Recovery, "✓"
	case "apply":
		result := summarizeKubernetesRollout(activity)
		if result.Controllers > 0 && result.ControllersReady == result.Controllers && (result.Pods == 0 || result.PodsReady == result.Pods) && result.Incidents == 0 {
			color, symbol = ui.Recovery, "✓"
		} else {
			color, symbol = ui.State, "◇"
		}
	}
	fields := []cardField{{Label: displayResourceType(activity.ResourceType), Value: activity.Resource}}
	if activity.EventType == "apply" && activity.ResourceType == "namespace" {
		result := summarizeKubernetesRollout(activity)
		if result.Controllers > 0 {
			fields = append(fields, cardField{Label: "Controllers", Value: fmt.Sprintf("%d/%d observed available", result.ControllersReady, result.Controllers)})
		}
		if result.Pods > 0 {
			fields = append(fields, cardField{Label: "Pods", Value: fmt.Sprintf("%d/%d observed ready", result.PodsReady, result.Pods)})
		}
		if !result.Last.IsZero() && activity.Command != nil {
			fields = append(fields, cardField{Label: "Observed", Value: result.Last.Sub(activity.Command.StartedAt).Round(time.Second).String()})
		}
		// An apply command's exit code only confirms API acceptance; success
		// here is restricted to the resource states observed by our collector.
		status := "Partial observations"
		if result.Controllers > 0 && result.Controllers == result.ControllersReady &&
			(result.Pods == 0 || result.Pods == result.PodsReady) {
			status = "All observed resources ready"
		}
		if result.Incidents > 0 {
			fields = append(fields, cardField{Label: "Alerts", Value: fmt.Sprintf("%d observed failure transitions", result.Incidents)})
			if status == "All observed resources ready" {
				status = "Ready now; review alerts"
			}
		}
		fields = append(fields, cardField{Label: "Status", Value: status})
		fields = append(fields, cardField{Label: "Attribution", Value: "Namespace/time correlation"})
	} else {
		var effects strings.Builder
		for i, effect := range summarizeActivityEffects(activity.Effects) {
			if i > 0 {
				effects.WriteByte('\n')
			}
			fmt.Fprintf(&effects, "%s: %s\n", displayResourceType(effect.ResourceType), effect.Resource)
			for _, line := range formatActivityEffectLines(effect) {
				fmt.Fprintf(&effects, "  %s\n", line)
			}
		}
		fields = append(fields, cardField{Label: "Effects", Value: strings.TrimSuffix(effects.String(), "\n")})
	}
	if activity.Command != nil {
		fields = append(fields, cardField{Label: "Command", Value: activity.Command.Command})
	}
	printCollectorCard(renderer, activity.OccurredAt, activity.Source,
		strings.ToUpper(strings.ReplaceAll(activity.EventType, "_", " ")), color, symbol, fields)
}
