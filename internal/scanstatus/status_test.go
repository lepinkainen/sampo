package scanstatus

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestStateSnapshotsConcurrentUpdates(t *testing.T) {
	state := New("root-1", "photos", 1000)

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				state.AddCompleted(1)
				state.AddError(1)
				_ = state.Snapshot()
			}
		}()
	}
	wg.Wait()
	state.Complete()

	snapshot := state.Snapshot()
	if snapshot.Running {
		t.Fatal("scan should be complete")
	}
	if snapshot.RootID != "root-1" || snapshot.Path != "photos" || snapshot.Total != 1000 {
		t.Fatalf("snapshot metadata changed: %+v", snapshot)
	}
	if snapshot.Completed != 1000 || snapshot.Errors != 1000 {
		t.Fatalf("snapshot counters = completed %d, errors %d; want 1000, 1000", snapshot.Completed, snapshot.Errors)
	}
}

func TestStaleStateDoesNotMutateCurrentState(t *testing.T) {
	var current atomic.Pointer[State]

	first := New("root-1", "first", 10)
	current.Store(first)

	second := New("root-1", "second", 20)
	current.Store(second)

	first.RecordError()
	first.AddCompleted(1)
	first.Complete()

	snapshot := current.Load().Snapshot()
	if !snapshot.Running {
		t.Fatal("current scan should still be running")
	}
	if snapshot.Path != "second" {
		t.Fatalf("current path = %q; want second", snapshot.Path)
	}
	if snapshot.Completed != 0 || snapshot.Errors != 0 {
		t.Fatalf("current counters changed: completed %d, errors %d", snapshot.Completed, snapshot.Errors)
	}
}
