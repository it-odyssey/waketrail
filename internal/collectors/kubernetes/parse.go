package kubernetes

import "encoding/json"

func parsePod(
	raw json.RawMessage,
) (PodState, error) {
	var object struct {
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`

		Status struct {
			Phase string `json:"phase"`

			ContainerStatuses []struct {
				Ready bool `json:"ready"`

				State struct {
					Waiting *struct {
						Reason string `json:"reason"`
					} `json:"waiting"`

					Terminated *struct {
						Reason string `json:"reason"`
					} `json:"terminated"`
				} `json:"state"`
			} `json:"containerStatuses"`
		} `json:"status"`
	}

	if err := json.Unmarshal(raw, &object); err != nil {
		return PodState{}, err
	}

	state := PodState{
		Namespace: object.Metadata.Namespace,
		Name:      object.Metadata.Name,
		Phase:     object.Status.Phase,
		Total:     len(object.Status.ContainerStatuses),
	}

	for _, container := range object.Status.ContainerStatuses {
		if container.Ready {
			state.Ready++
		}

		if state.Reason == "" &&
			container.State.Waiting != nil {
			state.Reason =
				container.State.Waiting.Reason
		}

		if state.Reason == "" &&
			container.State.Terminated != nil {
			state.Reason =
				container.State.Terminated.Reason
		}
	}

	return state, nil
}

func parseDeployment(
	raw json.RawMessage,
) (DeploymentState, error) {
	var object struct {
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`

		Spec struct {
			Replicas int32 `json:"replicas"`
		} `json:"spec"`

		Status struct {
			UpdatedReplicas   int32 `json:"updatedReplicas"`
			ReadyReplicas     int32 `json:"readyReplicas"`
			AvailableReplicas int32 `json:"availableReplicas"`
		} `json:"status"`
	}

	if err := json.Unmarshal(raw, &object); err != nil {
		return DeploymentState{}, err
	}

	return DeploymentState{
		Namespace: object.Metadata.Namespace,
		Name:      object.Metadata.Name,
		Desired:   object.Spec.Replicas,
		Updated:   object.Status.UpdatedReplicas,
		Ready:     object.Status.ReadyReplicas,
		Available: object.Status.AvailableReplicas,
	}, nil
}

func parseStatefulSet(
	raw json.RawMessage,
) (StatefulSetState, error) {
	var object struct {
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`

		Spec struct {
			Replicas int32 `json:"replicas"`
		} `json:"spec"`

		Status struct {
			CurrentReplicas int32 `json:"currentReplicas"`
			UpdatedReplicas int32 `json:"updatedReplicas"`
			ReadyReplicas   int32 `json:"readyReplicas"`
		} `json:"status"`
	}

	if err := json.Unmarshal(raw, &object); err != nil {
		return StatefulSetState{}, err
	}

	return StatefulSetState{
		Namespace: object.Metadata.Namespace,
		Name:      object.Metadata.Name,
		Desired:   object.Spec.Replicas,
		Current:   object.Status.CurrentReplicas,
		Updated:   object.Status.UpdatedReplicas,
		Ready:     object.Status.ReadyReplicas,
	}, nil
}

func parseDaemonSet(
	raw json.RawMessage,
) (DaemonSetState, error) {
	var object struct {
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`

		Status struct {
			DesiredNumberScheduled int32 `json:"desiredNumberScheduled"`
			CurrentNumberScheduled int32 `json:"currentNumberScheduled"`
			UpdatedNumberScheduled int32 `json:"updatedNumberScheduled"`
			NumberReady            int32 `json:"numberReady"`
			NumberAvailable        int32 `json:"numberAvailable"`
			NumberMisscheduled     int32 `json:"numberMisscheduled"`
		} `json:"status"`
	}

	if err := json.Unmarshal(raw, &object); err != nil {
		return DaemonSetState{}, err
	}

	return DaemonSetState{
		Namespace:    object.Metadata.Namespace,
		Name:         object.Metadata.Name,
		Desired:      object.Status.DesiredNumberScheduled,
		Current:      object.Status.CurrentNumberScheduled,
		Updated:      object.Status.UpdatedNumberScheduled,
		Ready:        object.Status.NumberReady,
		Available:    object.Status.NumberAvailable,
		Misscheduled: object.Status.NumberMisscheduled,
	}, nil
}

func parseNode(
	raw json.RawMessage,
) (NodeState, error) {
	var object struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`

		Spec struct {
			Unschedulable bool `json:"unschedulable"`
		} `json:"spec"`

		Status struct {
			Conditions []struct {
				Type   string `json:"type"`
				Status string `json:"status"`
			} `json:"conditions"`
		} `json:"status"`
	}

	if err := json.Unmarshal(raw, &object); err != nil {
		return NodeState{}, err
	}

	state := NodeState{
		Name:          object.Metadata.Name,
		Unschedulable: object.Spec.Unschedulable,
		Ready:         "Unknown",
	}

	for _, condition := range object.Status.Conditions {
		switch condition.Type {
		case "Ready":
			state.Ready = condition.Status

		case "MemoryPressure":
			state.MemoryPressure =
				condition.Status == "True"

		case "DiskPressure":
			state.DiskPressure =
				condition.Status == "True"

		case "PIDPressure":
			state.PIDPressure =
				condition.Status == "True"
		}
	}

	return state, nil
}
