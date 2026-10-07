package cmd

import (
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/it-odyssey/waketrail/internal/storage"
	"github.com/it-odyssey/waketrail/internal/ui"
)

// podObservation is a read-only interpretation of captured kubectl output.
// It is not a Kubernetes watch event and is never inserted into SQLite.
type podObservation struct {
	Namespace                       string
	Total, Ready, Running, Restarts int
	Problems                        []string
}

func parsePodObservation(command storage.CommandEvent, output storage.CommandOutput) (podObservation, bool) {
	if command.ExitCode != 0 || output.StdoutTruncated || output.StderrTruncated {
		return podObservation{}, false
	}
	fields := strings.Fields(command.Command)
	fields = stripCommandPrefixes(fields)
	if len(fields) < 3 || filepath.Base(fields[0]) != "kubectl" {
		return podObservation{}, false
	}
	operation := -1
	for i := 1; i < len(fields); i++ {
		if fields[i] == "get" {
			operation = i
			break
		}
		if fields[i] == "--" {
			break
		}
	}
	if operation < 0 || operation+1 >= len(fields) {
		return podObservation{}, false
	}
	resource := fields[operation+1]
	if resource != "pods" && resource != "pod" && resource != "po" {
		return podObservation{}, false
	}
	// Do not conflate a stream with a single point-in-time snapshot.
	for i := 1; i < len(fields); i++ {
		switch fields[i] {
		case "-w", "--watch", "--watch-only", "-o", "--output":
			if fields[i] == "-w" || strings.HasPrefix(fields[i], "--watch") {
				return podObservation{}, false
			}
			if i+1 >= len(fields) || fields[i+1] != "wide" {
				return podObservation{}, false
			}
		case "-A", "--all-namespaces", "--no-headers":
			return podObservation{}, false
		}
		if strings.HasPrefix(fields[i], "--watch=") || strings.HasPrefix(fields[i], "--output=") && fields[i] != "--output=wide" || strings.HasPrefix(fields[i], "-o=") && fields[i] != "-o=wide" {
			return podObservation{}, false
		}
		if strings.HasPrefix(fields[i], "-o") && len(fields[i]) > 2 && fields[i] != "-owide" {
			return podObservation{}, false
		}
	}
	lines := strings.Split(strings.TrimSpace(output.Stdout), "\n")
	if len(lines) < 2 {
		return podObservation{}, false
	}
	headers := strings.Fields(lines[0])
	columns := map[string]int{}
	for i, name := range headers {
		columns[name] = i
	}
	for _, name := range []string{"NAME", "READY", "STATUS", "RESTARTS"} {
		if _, ok := columns[name]; !ok {
			return podObservation{}, false
		}
	}
	obs := podObservation{Namespace: namespaceFromKubectlFields(fields)}
	if obs.Namespace == "" {
		obs.Namespace = "current context"
	}
	for _, line := range lines[1:] {
		row := strings.Fields(line)
		for _, name := range []string{"NAME", "READY", "STATUS", "RESTARTS"} {
			if columns[name] >= len(row) {
				return podObservation{}, false
			}
		}
		counts := strings.Split(row[columns["READY"]], "/")
		if len(counts) != 2 {
			return podObservation{}, false
		}
		ready, err := strconv.Atoi(counts[0])
		if err != nil {
			return podObservation{}, false
		}
		desired, err := strconv.Atoi(counts[1])
		if err != nil || ready < 0 || desired < 1 || ready > desired {
			return podObservation{}, false
		}
		restarts, err := strconv.Atoi(row[columns["RESTARTS"]])
		if err != nil || restarts < 0 {
			return podObservation{}, false
		}
		state := row[columns["STATUS"]]
		obs.Total++
		if ready == desired {
			obs.Ready++
		}
		if state == "Running" {
			obs.Running++
		}
		obs.Restarts += restarts
		if state != "Running" && state != "Completed" && state != "ContainerCreating" && state != "PodInitializing" && state != "Pending" || restarts > 0 {
			if len(obs.Problems) < 3 {
				obs.Problems = append(obs.Problems, fmt.Sprintf("%s: %s (%d restarts)", row[columns["NAME"]], state, restarts))
			}
		}
	}
	if obs.Total == 0 {
		return podObservation{}, false
	}
	return obs, true
}

func (o podObservation) Fields(command string) []cardField {
	status := "Observed, not all ready"
	if o.Ready == o.Total && len(o.Problems) == 0 {
		status = "All observed Pods ready"
	}
	fields := []cardField{
		{Label: "Namespace", Value: o.Namespace},
		{Label: "Pods", Value: fmt.Sprintf("%d/%d ready", o.Ready, o.Total)},
		{Label: "Running", Value: fmt.Sprintf("%d", o.Running)},
		{Label: "Restarts", Value: fmt.Sprintf("%d", o.Restarts)},
		{Label: "Status", Value: status},
	}
	if len(o.Problems) > 0 {
		fields = append(fields, cardField{Label: "Attention", Value: strings.Join(o.Problems, "; ")})
	}
	return append(fields, cardField{Label: "Command", Value: command})
}

// A semantic command card is offered only when the stored output is complete
// and the parser recognizes the exact table shape. Otherwise ordinary command
// rendering preserves the original evidence and its capture warnings.
func printPodObservation(renderer *lipgloss.Renderer, store *storage.Store, command storage.CommandEvent) bool {
	output, err := store.CommandOutputForCommandEvent(command.ID)
	if err != nil {
		return false
	}
	obs, ok := parsePodObservation(command, output)
	if !ok {
		return false
	}
	color, symbol := ui.State, "◇"
	if obs.Ready == obs.Total && len(obs.Problems) == 0 {
		color, symbol = ui.Recovery, "✓"
	}
	printCollectorCard(renderer, command.StartedAt, "kubernetes", "STATUS", color, symbol, obs.Fields(command.Command))
	return true
}

func writeMarkdownPodObservation(
	writer io.Writer,
	store *storage.Store,
	command storage.CommandEvent,
) bool {
	output, err := store.CommandOutputForCommandEvent(command.ID)
	if err != nil {
		return false
	}

	obs, ok := parsePodObservation(command, output)
	if !ok {
		return false
	}

	fmt.Fprintln(writer, "### Kubernetes: STATUS")
	fmt.Fprintln(writer)

	for _, field := range obs.Fields(command.Command) {
		fmt.Fprintf(
			writer,
			"- **%s:** %s\n",
			field.Label,
			field.Value,
		)
	}

	fmt.Fprintln(writer)
	fmt.Fprintln(
		writer,
		"*Snapshot derived from complete stored command output.*",
	)
	fmt.Fprintln(writer)

	return true
}
