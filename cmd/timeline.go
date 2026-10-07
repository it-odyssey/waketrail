package cmd

import (
	"github.com/it-odyssey/waketrail/internal/storage"
	"sort"
)

// buildDisplayEvents is the single reconstruction path used by show and export.
// Stored command and timeline rows are never modified by presentation logic.
func buildDisplayEvents(commands []storage.CommandEvent, timeline []storage.TimelineEvent, verbose bool) []displayEvent {
	events := make([]displayEvent, 0, len(commands)+len(timeline))
	for i := range commands {
		command := &commands[i]
		events = append(events, displayEvent{OccurredAt: command.StartedAt, Kind: "command", CommandEvent: command})
	}
	for i := range timeline {
		event := &timeline[i]
		events = append(events, displayEvent{OccurredAt: event.OccurredAt, Kind: "timeline", TimelineEvent: event})
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].OccurredAt.Before(events[j].OccurredAt) })
	events = correlateLifecycleEvents(events)
	if !verbose {
		events = correlateTerraformEvents(events)
		events = groupKubernetesActivities(events)
		events = filterWakeTrailCommands(events)
	}
	return events
}
