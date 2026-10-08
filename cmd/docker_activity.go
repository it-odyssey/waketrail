package cmd

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/it-odyssey/waketrail/internal/storage"
)

// dockerEffectSummary separates optional collector metadata from the observed
// transition. Older rows without Compose labels retain their original meaning.
func dockerEffectSummary(summary string) (clean, project, service string) {
	clean = summary
	const marker = " [compose:"
	i := strings.LastIndex(summary, marker)
	if i < 0 || !strings.HasSuffix(summary, "]") {
		return
	}
	tag := strings.TrimSuffix(summary[i+len(marker):], "]")
	parts := strings.SplitN(tag, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return
	}
	return summary[:i], parts[0], parts[1]
}

func composeCommand(command string) (action, project string, ok bool) {
	fields := stripCommandPrefixes(strings.Fields(command))
	if len(fields) < 3 || filepath.Base(fields[0]) != "docker" {
		return
	}
	i := 1
	for i < len(fields) && strings.HasPrefix(fields[i], "-") {
		i++
	}
	if i >= len(fields) || fields[i] != "compose" {
		return
	}
	i++
	for i < len(fields) {
		switch fields[i] {
		case "-p", "--project-name":
			if i+1 >= len(fields) {
				return "", "", false
			}
			project = fields[i+1]
			i += 2
			continue
		case "-f", "--file", "--env-file", "--profile", "--project-directory":
			if i+1 >= len(fields) {
				return "", "", false
			}
			i += 2
			continue
		}
		if strings.HasPrefix(fields[i], "--project-name=") {
			project = strings.TrimPrefix(fields[i], "--project-name=")
			i++
			continue
		}
		if strings.HasPrefix(fields[i], "-p=") {
			project = strings.TrimPrefix(fields[i], "-p=")
			i++
			continue
		}
		if strings.HasPrefix(fields[i], "-") {
			i++
			continue
		}
		switch fields[i] {
		case "up", "down", "start", "stop", "restart", "rm":
			return fields[i], project, true
		}
		return "", "", false
	}
	return "", "", false
}

// groupDockerActivities reconstructs two high-confidence cases: a named
// docker run/create followed by observed startup, and a Compose operation
// whose container events carry matching Compose labels.
// No watcher events means no inferred activity.
func groupDockerActivities(events []displayEvent) []displayEvent {
	used := make(map[int]bool)
	replacements := make(map[int]displayEvent)
	for i, e := range events {
		if e.Kind != "timeline" || e.TimelineEvent == nil || e.TimelineEvent.Source != "docker" || normalizedTimelineEventType(*e.TimelineEvent) != "created" || e.CorrelatedCommand == nil {
			continue
		}
		fields := stripCommandPrefixes(strings.Fields(e.CorrelatedCommand.Command))
		if len(fields) < 2 || fields[1] != "run" || !dockerCommandCreatesContainer(fields, timelineResourceName(*e.TimelineEvent)) {
			continue
		}
		matched := []storage.TimelineEvent{*e.TimelineEvent}
		for j := i + 1; j < len(events); j++ {
			next := events[j]
			if next.OccurredAt.Sub(e.OccurredAt) > 12*time.Second {
				break
			}
			if used[j] || next.Kind != "timeline" || next.TimelineEvent == nil || next.TimelineEvent.Source != "docker" || next.TimelineEvent.Resource != e.TimelineEvent.Resource {
				continue
			}
			if next.TimelineEvent.EventType != "state_change" {
				continue
			}
			clean, _, _ := dockerEffectSummary(timelineDisplaySummary(*next.TimelineEvent))
			if !strings.Contains(clean, "created → running") && !strings.Contains(clean, "created -> running") {
				continue
			}
			matched = append(matched, *next.TimelineEvent)
			used[j] = true
			break
		}
		if len(matched) < 2 {
			continue
		}
		a := displayActivity{OccurredAt: e.OccurredAt, Source: "docker", EventType: "created", ResourceType: "container", Resource: e.TimelineEvent.Resource, Command: e.CorrelatedCommand, Effects: matched}
		replacements[i] = displayEvent{OccurredAt: e.OccurredAt, Kind: "activity", Activity: &a}
	}
	// Compose groups must be keyed to actual project identity. Explicit -p
	// is strongest; a conventional directory name is accepted only if it
	// matches the label exactly, never by mere proximity alone.
	for i, e := range events {
		if e.Kind != "command" || e.CommandEvent == nil || e.CommandEvent.ExitCode != 0 {
			continue
		}
		action, explicitProject, ok := composeCommand(e.CommandEvent.Command)
		if !ok {
			continue
		}

		// Collect project-labelled Docker events in the operation window first.
		// When Compose gets its project name from `name:` in compose.yaml, the
		// project can legitimately differ from the working-directory basename.
		// We therefore infer only when exactly one observed Compose project is
		// present. Multiple projects remain ungrouped unless -p named one.
		candidatesByProject := make(map[string][]int)
		for j := i + 1; j < len(events); j++ {
			c := events[j]
			if c.OccurredAt.Sub(e.CommandEvent.EndedAt) > 25*time.Second {
				break
			}
			if c.Kind == "command" && c.CommandEvent != nil {
				if _, _, other := composeCommand(c.CommandEvent.Command); other {
					break
				}
			}
			if used[j] || c.Kind != "timeline" || c.TimelineEvent == nil || c.TimelineEvent.Source != "docker" || c.OccurredAt.Before(e.CommandEvent.StartedAt) {
				continue
			}
			_, project, _ := dockerEffectSummary(c.TimelineEvent.Summary)
			if project == "" {
				continue
			}
			candidatesByProject[project] = append(candidatesByProject[project], j)
		}

		project := explicitProject
		if project == "" {
			if len(candidatesByProject) == 1 {
				for observedProject := range candidatesByProject {
					project = observedProject
				}
			} else {
				// Directory naming is only a disambiguator when that exact project
				// was actually observed; it is never used as evidence by itself.
				cwdProject := filepath.Base(filepath.Clean(e.CommandEvent.Cwd))
				if _, found := candidatesByProject[cwdProject]; found {
					project = cwdProject
				}
			}
		}
		if project == "" {
			continue
		}
		matches := candidatesByProject[project]
		if len(matches) == 0 {
			continue
		}
		effects := make([]storage.TimelineEvent, 0, len(matches))
		for _, j := range matches {
			used[j] = true
			effects = append(effects, *events[j].TimelineEvent)
		}
		activity := displayActivity{OccurredAt: e.CommandEvent.StartedAt, Source: "docker compose", EventType: action, ResourceType: "deployment", Resource: project, Command: e.CommandEvent, Effects: effects}
		for _, effect := range effects {
			if normalizedTimelineEventType(effect) == "failure" {
				activity.HasObservedFailure = true
			}
		}
		activity.Transitions = summarizeComposeTransitions(effects)
		activity.EffectScope = "Observed affected containers/services"
		replacements[i] = displayEvent{OccurredAt: activity.OccurredAt, Kind: "activity", Activity: &activity}
	}
	result := make([]displayEvent, 0, len(events))
	for i, e := range events {
		if r, ok := replacements[i]; ok {
			result = append(result, r)
			continue
		}
		if !used[i] {
			result = append(result, e)
		}
	}
	return result
}

// composeContainerEndpoints describes only the affected containers, not a
// project inventory. An empty state is unknown; "absent" is observed absence.
type composeContainerEndpoints struct {
	before, after string
	service       string
}

// summarizeComposeTransitions reconstructs endpoints from stored evidence.
// Removal rows historically store only "disappeared", so they establish
// presence/absence but cannot by themselves establish a prior running state.
func summarizeComposeTransitions(effects []storage.TimelineEvent) []activityTransition {
	containers := make(map[string]composeContainerEndpoints)
	created, removed := make(map[string]bool), make(map[string]bool)
	failures, recoveries := 0, 0
	for _, e := range effects {
		clean, _, service := dockerEffectSummary(timelineDisplaySummary(e))
		clean = strings.TrimPrefix(clean, e.Resource+" ")
		clean = formatTransitionArrow(clean)
		before, after := "", ""
		switch {
		case strings.HasPrefix(clean, "appeared: "):
			before, after = "absent", strings.TrimPrefix(clean, "appeared: ")
			created[e.Resource] = true
		case clean == "disappeared":
			after = "absent"
			removed[e.Resource] = true
		default:
			parts := strings.SplitN(clean, " → ", 2)
			if len(parts) == 2 {
				before, after = parts[0], parts[1]
			}
		}
		endpoints, seen := containers[e.Resource]
		if !seen {
			endpoints.before = before
		}
		endpoints.after = after
		endpoints.service = service
		containers[e.Resource] = endpoints
		switch normalizedTimelineEventType(e) {
		case "failure":
			failures++
		case "recovery":
			recoveries++
		}
	}

	var result []activityTransition
	appendChange := func(name string, before, after int) {
		if before != after {
			result = append(result, activityTransition{Name: name, Before: before, After: after})
		}
	}
	appendChange("created", 0, len(created))
	// Omit a state count if any affected container's endpoint is unknown.
	// A missing health suffix also cannot establish a health-check result.
	countState := func(healthy bool) (before, after int, known bool) {
		known = true
		for _, c := range containers {
			for i, state := range []string{c.before, c.after} {
				parts := strings.SplitN(state, "/", 2)
				if state == "" || state == "unknown" || (healthy && state != "absent" && len(parts) != 2) {
					known = false
				}
				matches := parts[0] == "running"
				if healthy {
					matches = len(parts) == 2 && parts[1] == "healthy"
				}
				if matches {
					if i == 0 {
						before++
					} else {
						after++
					}
				}
			}
		}
		return
	}
	if before, after, known := countState(false); known {
		appendChange("running", before, after)
	}
	appendChange("removed", 0, len(removed))
	if before, after, known := countState(true); known {
		appendChange("healthy", before, after)
	}
	appendChange("failures", 0, failures)
	appendChange("recoveries", 0, recoveries)

	// Services count distinct labelled services with affected containers
	// present at each endpoint, including stopped containers. Replicas are
	// not services, and unaffected services are outside this observation.
	servicesBefore, servicesAfter := make(map[string]bool), make(map[string]bool)
	for _, c := range containers {
		if c.service == "" {
			continue
		}
		if c.before != "absent" {
			servicesBefore[c.service] = true
		}
		if c.after != "absent" {
			servicesAfter[c.service] = true
		}
	}
	appendChange("services", len(servicesBefore), len(servicesAfter))
	return result
}
