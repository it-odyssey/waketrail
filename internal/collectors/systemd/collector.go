package systemd

import (
	"fmt"

	"github.com/it-odyssey/waketrail/internal/watch"
)

type Collector struct{}

func NewCollector() Collector {
	return Collector{}
}

func (Collector) Name() string {
	return "systemd"
}

func (Collector) Snapshot() (any, error) {
	return Detect()
}

func (Collector) Compare(
	previous any,
	current any,
) ([]watch.Event, error) {
	previousUnits, ok := previous.([]UnitState)
	if !ok {
		return nil, fmt.Errorf(
			"systemd collector received invalid previous snapshot",
		)
	}

	currentUnits, ok := current.([]UnitState)
	if !ok {
		return nil, fmt.Errorf(
			"systemd collector received invalid current snapshot",
		)
	}

	transitions := Compare(
		previousUnits,
		currentUnits,
	)

	events := make(
		[]watch.Event,
		0,
		len(transitions),
	)

	for _, transition := range transitions {
		events = append(events, watch.Event{
			EventType: transition.EventType,
			Source:    "systemd",
			Summary:   transition.Summary,
		})
	}

	return events, nil
}
