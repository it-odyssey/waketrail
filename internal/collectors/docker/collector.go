package docker

import (
	"fmt"

	"github.com/it-odyssey/waketrail/internal/watch"
)

type Collector struct{}

func NewCollector() Collector {
	return Collector{}
}

func (Collector) Name() string {
	return "docker"
}

func (Collector) Snapshot() (any, error) {
	return Detect()
}

func (Collector) Compare(
	previous any,
	current any,
) ([]watch.Event, error) {
	previousContainers, ok := previous.([]ContainerState)
	if !ok {
		return nil, fmt.Errorf(
			"docker collector received invalid previous snapshot",
		)
	}

	currentContainers, ok := current.([]ContainerState)
	if !ok {
		return nil, fmt.Errorf(
			"docker collector received invalid current snapshot",
		)
	}

	transitions := Compare(
		previousContainers,
		currentContainers,
	)

	events := make(
		[]watch.Event,
		0,
		len(transitions),
	)

	for _, transition := range transitions {
		events = append(events, watch.Event{
			EventType: transition.EventType,
			Source:    "docker",
			Summary:   transition.Summary,
		})
	}

	return events, nil
}
