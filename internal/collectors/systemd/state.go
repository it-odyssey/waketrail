package systemd

import (
	"bufio"
	"bytes"
	"errors"
	"os/exec"
	"strings"
)

type UnitState struct {
	Name      string
	LoadState string
	Active    string
	Sub       string
}

func Detect() ([]UnitState, error) {
	cmd := exec.Command(
		"systemctl",
		"list-units",
		"--type=service",
		"--all",
		"--no-legend",
		"--no-pager",
		"--plain",
	)

	output, err := cmd.Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, errors.New("systemctl CLI not found")
		}

		return nil, err
	}

	return parseSystemctl(output)
}

func parseSystemctl(output []byte) ([]UnitState, error) {
	var units []UnitState

	scanner := bufio.NewScanner(bytes.NewReader(output))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" {
			continue
		}

		fields := strings.Fields(line)

		if len(fields) < 4 {
			continue
		}

		units = append(units, UnitState{
			Name:      fields[0],
			LoadState: fields[1],
			Active:    fields[2],
			Sub:       fields[3],
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return units, nil
}
