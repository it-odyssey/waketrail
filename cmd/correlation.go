package cmd

import (
	"github.com/it-odyssey/waketrail/internal/storage"
	"path/filepath"
	"strings"
	"time"
)

// correlateLifecycleEvents attributes observed transitions to the most recent
// successful matching command. It does not manufacture collector events when
// the background watcher was absent or missed a short-lived transition.
func correlateLifecycleEvents(events []displayEvent) []displayEvent {
	const correlationWindow = 12 * time.Second
	matchedCommands := make(map[*storage.CommandEvent]bool)
	for i := range events {
		event := &events[i]
		if !isLifecycleEvent(*event) {
			continue
		}
		for j := i - 1; j >= 0; j-- {
			candidate := events[j]
			if candidate.Kind != "command" || candidate.CommandEvent == nil {
				continue
			}
			cmd := candidate.CommandEvent
			if event.OccurredAt.Sub(cmd.EndedAt) > correlationWindow {
				break
			}
			if cmd.ExitCode != 0 || event.OccurredAt.Before(cmd.EndedAt) {
				continue
			}
			if commandMatchesLifecycle(*cmd, *event.TimelineEvent) {
				event.CorrelatedCommand = cmd
				matchedCommands[cmd] = true
				break
			}
		}
	}
	result := make([]displayEvent, 0, len(events))
	for _, event := range events {
		if event.Kind == "command" && matchedCommands[event.CommandEvent] {
			continue
		}
		result = append(result, event)
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

	switch normalizedTimelineEventType(*event.TimelineEvent) {
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

	switch normalizedTimelineEventType(event) {
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
