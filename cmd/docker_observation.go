package cmd

import (
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/it-odyssey/waketrail/internal/storage"
	"github.com/it-odyssey/waketrail/internal/ui"
)

var dockerColumns = regexp.MustCompile(`\s{2,}`)

type dockerObservation struct {
	Scope                                                       string
	All                                                         bool
	Filtered                                                    bool
	Listed, Running, Stopped, Unhealthy, HealthChecked, Healthy int
}

// dockerPSCommand recognizes human-readable docker/compose ps invocations.
// Filters are safe to summarize, but they make the result a scoped listing
// rather than a host-wide inventory even when -a is present.
func dockerPSCommand(command string) (scope string, all, filtered, ok bool) {
	fields := stripCommandPrefixes(strings.Fields(command))
	if len(fields) < 2 || filepath.Base(fields[0]) != "docker" {
		return
	}

	i := 1
	compose := false
	if fields[i] == "compose" {
		compose = true
		scope = "Compose project (listed containers)"
		i++
		for i < len(fields) {
			switch fields[i] {
			case "-p", "--project-name":
				if i+1 >= len(fields) {
					return "", false, false, false
				}
				scope = "Compose project " + fields[i+1]
				i += 2
				continue
			case "-f", "--file", "--project-directory", "--env-file", "--profile":
				if i+1 >= len(fields) {
					return "", false, false, false
				}
				i += 2
				continue
			}
			if strings.HasPrefix(fields[i], "--project-name=") {
				scope = "Compose project " + strings.TrimPrefix(fields[i], "--project-name=")
				i++
				continue
			}
			if strings.HasPrefix(fields[i], "-") {
				i++
				continue
			}
			break
		}
	} else {
		scope = "Docker host"
	}

	if i >= len(fields) || fields[i] != "ps" {
		return "", false, false, false
	}

	for i = i + 1; i < len(fields); i++ {
		flag := fields[i]
		switch flag {
		case "-a", "--all":
			all = true
		case "-q", "--quiet", "--no-trunc":
			return "", false, false, false
		case "-f", "--filter", "--status":
			if i+1 >= len(fields) {
				return "", false, false, false
			}
			filtered = true
			i++
		case "--services":
			// Compose --services changes the output away from the table shape.
			return "", false, false, false
		default:
			switch {
			case strings.HasPrefix(flag, "--filter=") || strings.HasPrefix(flag, "--status="):
				filtered = true
			case strings.HasPrefix(flag, "--format") || strings.HasPrefix(flag, "-q"):
				return "", false, false, false
			case strings.HasPrefix(flag, "-") && strings.Contains(flag, "a") && len(flag) <= 4:
				all = true
			default:
				return "", false, false, false
			}
		}
	}

	if filtered {
		if compose {
			scope = "Compose project (filtered containers)"
		} else {
			scope = "Filtered containers"
		}
	}
	return scope, all, filtered, true
}

// parseDockerObservation accepts only the standard human-readable ps table.
// Quiet/custom formats, malformed tables, failed commands, and incomplete
// captured output fall back to the original command rather than invent totals.
func parseDockerObservation(command storage.CommandEvent, output storage.CommandOutput) (dockerObservation, bool) {
	scope, all, filtered, ok := dockerPSCommand(command.Command)
	if !ok || command.ExitCode != 0 || output.StdoutTruncated || output.StderrTruncated {
		return dockerObservation{}, false
	}
	raw := strings.TrimSpace(output.Stdout)
	if raw == "" {
		return dockerObservation{}, false
	}
	lines := strings.Split(raw, "\n")
	heads := dockerColumns.Split(strings.TrimSpace(lines[0]), -1)
	indices := map[string]int{}
	for i, h := range heads {
		indices[h] = i
	}
	if _, hasName := indices["NAME"]; !hasName {
		if _, hasNames := indices["NAMES"]; !hasNames {
			return dockerObservation{}, false
		}
	}
	statusIdx, hasStatus := indices["STATUS"]
	if !hasStatus {
		return dockerObservation{}, false
	}

	o := dockerObservation{Scope: scope, All: all, Filtered: filtered}
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		cells := dockerColumns.Split(strings.TrimSpace(line), -1)
		if statusIdx >= len(cells) {
			return dockerObservation{}, false
		}
		status := cells[statusIdx]
		o.Listed++
		if strings.HasPrefix(status, "Up ") || strings.EqualFold(status, "running") || strings.HasPrefix(strings.ToLower(status), "running ") {
			o.Running++
		} else {
			o.Stopped++
		}
		if strings.Contains(status, "(healthy)") {
			o.Healthy++
			o.HealthChecked++
		}
		if strings.Contains(status, "(unhealthy)") {
			o.Unhealthy++
			o.HealthChecked++
		}
	}
	return o, true
}

func (o dockerObservation) Fields(command string) []cardField {
	label := "Listed"
	if o.All && !o.Filtered {
		label = "Containers"
	}
	fields := []cardField{
		{Label: "Scope", Value: o.Scope},
		{Label: label, Value: fmt.Sprintf("%d observed", o.Listed)},
		{Label: "Running", Value: fmt.Sprintf("%d", o.Running)},
		{Label: "Not running", Value: fmt.Sprintf("%d", o.Stopped)},
	}
	if o.HealthChecked > 0 {
		fields = append(fields, cardField{Label: "Health", Value: fmt.Sprintf("%d/%d healthy (health checks reported)", o.Healthy, o.HealthChecked)})
	}

	status := "Snapshot of listed containers; not a host-wide inventory"
	switch {
	case o.Filtered:
		status = "Snapshot of filtered containers; totals apply only to the filter"
	case o.All:
		status = "Snapshot includes stopped containers"
	}
	fields = append(fields, cardField{Label: "Status", Value: status})
	return append(fields, cardField{Label: "Command", Value: command})
}

func printDockerObservation(renderer *lipgloss.Renderer, store *storage.Store, command storage.CommandEvent) bool {
	output, err := store.CommandOutputForCommandEvent(command.ID)
	if err != nil {
		return false
	}
	o, ok := parseDockerObservation(command, output)
	if !ok {
		return false
	}
	color, symbol := ui.State, "◇"
	if o.Unhealthy > 0 {
		color, symbol = ui.Failure, "✗"
	}
	source := "docker"
	if strings.HasPrefix(o.Scope, "Compose project") {
		source = "docker compose"
	}
	printCollectorCard(renderer, command.StartedAt, source, "STATUS", color, symbol, o.Fields(command.Command))
	return true
}

func writeMarkdownDockerObservation(writer io.Writer, store *storage.Store, command storage.CommandEvent) bool {
	output, err := store.CommandOutputForCommandEvent(command.ID)
	if err != nil {
		return false
	}
	o, ok := parseDockerObservation(command, output)
	if !ok {
		return false
	}
	source := "Docker"
	if strings.HasPrefix(o.Scope, "Compose project") {
		source = "Docker Compose"
	}
	fmt.Fprintln(writer, "### "+source+": STATUS")
	fmt.Fprintln(writer)
	for _, field := range o.Fields(command.Command) {
		fmt.Fprintf(writer, "- **%s:** %s\n", field.Label, field.Value)
	}
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "*Snapshot derived from complete stored command output.*")
	fmt.Fprintln(writer)
	return true
}
