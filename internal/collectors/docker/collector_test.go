package docker

import "testing"

func TestCollectorCompareNormalizesDockerTransitions(
	t *testing.T,
) {
	collector := NewCollector()

	previous := []ContainerState{
		{
			Name:   "nginx",
			State:  "running",
			Status: "Up 10 minutes",
		},
	}

	current := []ContainerState{
		{
			Name:   "nginx",
			State:  "exited",
			Status: "Exited (0) 2 seconds ago",
		},
	}

	events, err := collector.Compare(
		previous,
		current,
	)
	if err != nil {
		t.Fatalf(
			"Compare() returned error: %v",
			err,
		)
	}

	if len(events) != 1 {
		t.Fatalf(
			"len(events) = %d, want 1",
			len(events),
		)
	}

	if events[0].Source != "docker" {
		t.Errorf(
			"Source = %q, want %q",
			events[0].Source,
			"docker",
		)
	}

	if events[0].EventType != EventStopped {
		t.Errorf(
			"EventType = %q, want %q",
			events[0].EventType,
			EventStopped,
		)
	}
}
