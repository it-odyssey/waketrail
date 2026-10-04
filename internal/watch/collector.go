package watch

// Event is a normalized state change produced by any WakeTrail collector.
// Collectors translate their domain-specific state into this common format
// before the event reaches storage or presentation.
type Event struct {
	EventType    string
	Source       string
	ResourceType string
	Resource     string
	Summary      string
}

// Collector represents a system that WakeTrail can observe continuously.
//
// Snapshot returns collector-specific state. Compare receives two snapshots
// from the same collector and translates any differences into WakeTrail
// events.
type Collector interface {
	Name() string
	Snapshot() (any, error)
	Compare(previous any, current any) ([]Event, error)
}
