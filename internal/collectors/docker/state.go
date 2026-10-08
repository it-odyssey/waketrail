package docker

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

type ContainerState struct {
	Name           string
	State          string
	Status         string
	Health         string
	ComposeProject string
	ComposeService string
}

func Detect() ([]ContainerState, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx,
		"docker",
		"ps",
		"-a",
		"--format",
		`{{.Names}}	{{.State}}	{{.Status}}	{{.Label "com.docker.compose.project"}}	{{.Label "com.docker.compose.service"}}`,
	)

	output, err := cmd.Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, errors.New("docker CLI not found")
		}

		return nil, err
	}

	return parseDockerPS(output)
}

func parseDockerPS(output []byte) ([]ContainerState, error) {
	var containers []ContainerState

	scanner := bufio.NewScanner(bytes.NewReader(output))

	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")

		if strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Split(line, "\t")

		if len(fields) != 3 && len(fields) != 5 {
			continue
		}

		status := fields[2]

		project, service := "", ""
		if len(fields) == 5 {
			project = strings.TrimSpace(fields[3])
			service = strings.TrimSpace(fields[4])
			if project == "<no value>" || project == "<nil>" {
				project = ""
			}
			if service == "<no value>" || service == "<nil>" {
				service = ""
			}
		}
		containers = append(containers, ContainerState{
			ComposeProject: project, ComposeService: service,
			Name:   fields[0],
			State:  fields[1],
			Status: status,
			Health: healthFromStatus(status),
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return containers, nil
}

func healthFromStatus(status string) string {
	switch {
	case strings.Contains(status, "(healthy)"):
		return "healthy"

	case strings.Contains(status, "(unhealthy)"):
		return "unhealthy"

	case strings.Contains(status, "(health: starting)"):
		return "starting"

	default:
		return ""
	}
}
