package scanstatus

import "sync/atomic"

// Snapshot reports scan progress for JSON APIs.
type Snapshot struct {
	Running   bool   `json:"running"`
	RootID    string `json:"rootId,omitempty"`
	Path      string `json:"path,omitempty"`
	Total     int64  `json:"total"`
	Completed int64  `json:"completed"`
	Errors    int64  `json:"errors"`
}

// State holds mutable scan progress. Counters and running state are atomic so
// HTTP polling can snapshot progress while workers update it.
type State struct {
	rootID string
	path   string
	total  int64

	running   atomic.Bool
	completed atomic.Int64
	errors    atomic.Int64
}

// NewIdle creates a non-running scan state.
func NewIdle() *State {
	return &State{}
}

// New creates a running scan state.
func New(rootID, path string, total int64) *State {
	state := &State{
		rootID: rootID,
		path:   path,
		total:  total,
	}
	state.running.Store(true)
	return state
}

// Complete marks this scan state as no longer running.
func (s *State) Complete() {
	if s == nil {
		return
	}
	s.running.Store(false)
}

// AddCompleted records completed items for this scan state.
func (s *State) AddCompleted(delta int64) {
	if s == nil {
		return
	}
	s.completed.Add(delta)
}

// AddError records errors for this scan state.
func (s *State) AddError(delta int64) {
	if s == nil {
		return
	}
	s.errors.Add(delta)
}

// RecordError records a failed item as both errored and completed.
func (s *State) RecordError() {
	if s == nil {
		return
	}
	s.errors.Add(1)
	s.completed.Add(1)
}

// Snapshot returns a race-free copy of scan progress.
func (s *State) Snapshot() Snapshot {
	if s == nil {
		return Snapshot{}
	}
	return Snapshot{
		Running:   s.running.Load(),
		RootID:    s.rootID,
		Path:      s.path,
		Total:     s.total,
		Completed: s.completed.Load(),
		Errors:    s.errors.Load(),
	}
}
