package kubernetes

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
)

type resourceList struct {
	Items []json.RawMessage `json:"items"`
}

type objectMeta struct {
	Kind string `json:"kind"`

	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
}

func Detect() (Snapshot, error) {
	cmd := exec.Command(
		"kubectl",
		"get",
		"pods,deployments,statefulsets,daemonsets,nodes",
		"--all-namespaces",
		"-o",
		"json",
	)

	output, err := cmd.Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return Snapshot{}, errors.New("kubectl CLI not found")
		}

		var exitErr *exec.ExitError

		if errors.As(err, &exitErr) {
			return Snapshot{}, fmt.Errorf(
				"kubectl failed: %s",
				string(exitErr.Stderr),
			)
		}

		return Snapshot{}, err
	}

	return parseSnapshot(output)
}

func parseSnapshot(
	output []byte,
) (Snapshot, error) {
	var list resourceList

	if err := json.Unmarshal(output, &list); err != nil {
		return Snapshot{}, err
	}

	var snapshot Snapshot

	for _, raw := range list.Items {
		var meta objectMeta

		if err := json.Unmarshal(raw, &meta); err != nil {
			return Snapshot{}, err
		}

		switch meta.Kind {
		case "Pod":
			state, err := parsePod(raw)
			if err != nil {
				return Snapshot{}, err
			}

			snapshot.Pods = append(
				snapshot.Pods,
				state,
			)

		case "Deployment":
			state, err := parseDeployment(raw)
			if err != nil {
				return Snapshot{}, err
			}

			snapshot.Deployments = append(
				snapshot.Deployments,
				state,
			)

		case "StatefulSet":
			state, err := parseStatefulSet(raw)
			if err != nil {
				return Snapshot{}, err
			}

			snapshot.StatefulSets = append(
				snapshot.StatefulSets,
				state,
			)

		case "DaemonSet":
			state, err := parseDaemonSet(raw)
			if err != nil {
				return Snapshot{}, err
			}

			snapshot.DaemonSets = append(
				snapshot.DaemonSets,
				state,
			)

		case "Node":
			state, err := parseNode(raw)
			if err != nil {
				return Snapshot{}, err
			}

			snapshot.Nodes = append(
				snapshot.Nodes,
				state,
			)
		}
	}

	return snapshot, nil
}
