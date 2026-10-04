package kubernetes

import "testing"

func TestComparePodFailure(t *testing.T) {
	previous := Snapshot{
		Pods: []PodState{
			{
				Namespace: "default",
				Name:      "api",
				Phase:     "Running",
				Ready:     1,
				Total:     1,
			},
		},
	}

	current := Snapshot{
		Pods: []PodState{
			{
				Namespace: "default",
				Name:      "api",
				Phase:     "Running",
				Ready:     0,
				Total:     1,
				Reason:    "CrashLoopBackOff",
			},
		},
	}

	transitions := Compare(previous, current)

	assertSingleTransition(
		t,
		transitions,
		"pod",
		"default/api",
		EventFailure,
	)
}

func TestComparePodRecovery(t *testing.T) {
	previous := Snapshot{
		Pods: []PodState{
			{
				Namespace: "default",
				Name:      "api",
				Phase:     "Running",
				Ready:     0,
				Total:     1,
				Reason:    "CrashLoopBackOff",
			},
		},
	}

	current := Snapshot{
		Pods: []PodState{
			{
				Namespace: "default",
				Name:      "api",
				Phase:     "Running",
				Ready:     1,
				Total:     1,
			},
		},
	}

	transitions := Compare(previous, current)

	assertSingleTransition(
		t,
		transitions,
		"pod",
		"default/api",
		EventRecovery,
	)
}

func TestCompareDeploymentFailure(t *testing.T) {
	previous := Snapshot{
		Deployments: []DeploymentState{
			{
				Namespace: "default",
				Name:      "api",
				Desired:   3,
				Updated:   3,
				Ready:     3,
				Available: 3,
			},
		},
	}

	current := Snapshot{
		Deployments: []DeploymentState{
			{
				Namespace: "default",
				Name:      "api",
				Desired:   3,
				Updated:   3,
				Ready:     2,
				Available: 2,
			},
		},
	}

	transitions := Compare(previous, current)

	assertSingleTransition(
		t,
		transitions,
		"deployment",
		"default/api",
		EventFailure,
	)
}

func TestCompareStatefulSetRecovery(t *testing.T) {
	previous := Snapshot{
		StatefulSets: []StatefulSetState{
			{
				Namespace: "data",
				Name:      "postgres",
				Desired:   3,
				Current:   3,
				Updated:   3,
				Ready:     2,
			},
		},
	}

	current := Snapshot{
		StatefulSets: []StatefulSetState{
			{
				Namespace: "data",
				Name:      "postgres",
				Desired:   3,
				Current:   3,
				Updated:   3,
				Ready:     3,
			},
		},
	}

	transitions := Compare(previous, current)

	assertSingleTransition(
		t,
		transitions,
		"statefulset",
		"data/postgres",
		EventRecovery,
	)
}

func TestCompareDaemonSetMisscheduled(t *testing.T) {
	previous := Snapshot{
		DaemonSets: []DaemonSetState{
			{
				Namespace: "monitoring",
				Name:      "agent",
				Desired:   3,
				Current:   3,
				Updated:   3,
				Ready:     3,
				Available: 3,
			},
		},
	}

	current := Snapshot{
		DaemonSets: []DaemonSetState{
			{
				Namespace:    "monitoring",
				Name:         "agent",
				Desired:      3,
				Current:      3,
				Updated:      3,
				Ready:        3,
				Available:    3,
				Misscheduled: 1,
			},
		},
	}

	transitions := Compare(previous, current)

	assertSingleTransition(
		t,
		transitions,
		"daemonset",
		"monitoring/agent",
		EventFailure,
	)
}

func TestCompareNodeNotReady(t *testing.T) {
	previous := Snapshot{
		Nodes: []NodeState{
			{
				Name:  "worker-1",
				Ready: "True",
			},
		},
	}

	current := Snapshot{
		Nodes: []NodeState{
			{
				Name:  "worker-1",
				Ready: "False",
			},
		},
	}

	transitions := Compare(previous, current)

	assertSingleTransition(
		t,
		transitions,
		"node",
		"worker-1",
		EventFailure,
	)
}

func TestCordonedNodeIsStateChangeNotFailure(
	t *testing.T,
) {
	previous := Snapshot{
		Nodes: []NodeState{
			{
				Name:  "worker-1",
				Ready: "True",
			},
		},
	}

	current := Snapshot{
		Nodes: []NodeState{
			{
				Name:          "worker-1",
				Ready:         "True",
				Unschedulable: true,
			},
		},
	}

	transitions := Compare(previous, current)

	assertSingleTransition(
		t,
		transitions,
		"node",
		"worker-1",
		EventStateChange,
	)
}

func assertSingleTransition(
	t *testing.T,
	transitions []Transition,
	resourceType string,
	resource string,
	eventType string,
) {
	t.Helper()

	if len(transitions) != 1 {
		t.Fatalf(
			"len(transitions) = %d, want 1: %+v",
			len(transitions),
			transitions,
		)
	}

	transition := transitions[0]

	if transition.ResourceType != resourceType {
		t.Errorf(
			"ResourceType = %q, want %q",
			transition.ResourceType,
			resourceType,
		)
	}

	if transition.Resource != resource {
		t.Errorf(
			"Resource = %q, want %q",
			transition.Resource,
			resource,
		)
	}

	if transition.EventType != eventType {
		t.Errorf(
			"EventType = %q, want %q",
			transition.EventType,
			eventType,
		)
	}
}
