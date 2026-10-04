package kubernetes

import (
	"fmt"

	"github.com/it-odyssey/waketrail/internal/watch"
)

type Collector struct{}

func NewCollector() Collector {
	return Collector{}
}

func (Collector) Name() string {
	return "kubernetes"
}

func (Collector) Snapshot() (any, error) {
	return Detect()
}

func (Collector) Compare(
	previous any,
	current any,
) ([]watch.Event, error) {
	previousSnapshot, ok := previous.(Snapshot)
	if !ok {
		return nil, fmt.Errorf(
			"kubernetes collector received invalid previous snapshot",
		)
	}

	currentSnapshot, ok := current.(Snapshot)
	if !ok {
		return nil, fmt.Errorf(
			"kubernetes collector received invalid current snapshot",
		)
	}

	transitions := Compare(
		previousSnapshot,
		currentSnapshot,
	)

	events := make(
		[]watch.Event,
		0,
		len(transitions),
	)

	for _, transition := range transitions {
		events = append(
			events,
			watch.Event{
				EventType:    transition.EventType,
				Source:       "kubernetes",
				ResourceType: transition.ResourceType,
				Resource:     transition.Resource,
				Summary:      transition.Summary,
			},
		)
	}

	return events, nil
}
