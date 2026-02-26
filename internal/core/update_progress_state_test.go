package core

import (
	"errors"
	"testing"
	"time"
)

func readUpdateSnapshot(t *testing.T, ch <-chan UpdateProgressSnapshot) UpdateProgressSnapshot {
	t.Helper()
	select {
	case snap := <-ch:
		return snap
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for update snapshot")
		return UpdateProgressSnapshot{}
	}
}

func TestUpdateProgressTryLockRejectsConcurrentBegin(t *testing.T) {
	state := NewUpdateProgressState()

	if ok := state.Begin("1.0.0"); !ok {
		t.Fatal("first Begin should acquire lock")
	}
	if ok := state.Begin("1.0.1"); ok {
		t.Fatal("second Begin should fail while update in progress")
	}

	snapshot := state.Snapshot()
	if !snapshot.InProgress {
		t.Fatal("snapshot should report in progress")
	}
	if snapshot.Version != "1.0.0" {
		t.Fatalf("snapshot version = %q, want %q", snapshot.Version, "1.0.0")
	}

	state.Finish(nil)
	if ok := state.Begin("1.0.1"); !ok {
		t.Fatal("Begin should succeed again after Finish")
	}
	state.Finish(nil)
}

func TestUpdateProgressBroadcastsConsistentSnapshots(t *testing.T) {
	state := NewUpdateProgressState()
	if ok := state.Begin("1.2.3"); !ok {
		t.Fatal("Begin should acquire lock")
	}

	idA, subA := state.Subscribe()
	idB, subB := state.Subscribe()
	defer state.Unsubscribe(idA)
	defer state.Unsubscribe(idB)

	initialA := readUpdateSnapshot(t, subA)
	initialB := readUpdateSnapshot(t, subB)
	if initialA.Percent != initialB.Percent || initialA.InProgress != initialB.InProgress {
		t.Fatalf("initial snapshots differ: A=%+v B=%+v", initialA, initialB)
	}

	state.Publish(25)
	p25A := readUpdateSnapshot(t, subA)
	p25B := readUpdateSnapshot(t, subB)
	if p25A.Percent != 25 || p25B.Percent != 25 {
		t.Fatalf("percent after Publish(25): A=%d B=%d, want 25", p25A.Percent, p25B.Percent)
	}

	state.Publish(60)
	p60A := readUpdateSnapshot(t, subA)
	p60B := readUpdateSnapshot(t, subB)
	if p60A.Percent != 60 || p60B.Percent != 60 {
		t.Fatalf("percent after Publish(60): A=%d B=%d, want 60", p60A.Percent, p60B.Percent)
	}

	state.Finish(nil)
	doneA := readUpdateSnapshot(t, subA)
	doneB := readUpdateSnapshot(t, subB)
	if doneA.InProgress || doneB.InProgress {
		t.Fatalf("final snapshots should not be in progress: A=%+v B=%+v", doneA, doneB)
	}
	if doneA.Percent != 100 || doneB.Percent != 100 {
		t.Fatalf("final snapshots should converge to 100: A=%d B=%d", doneA.Percent, doneB.Percent)
	}
}

func TestUpdateProgressFinishConvergesState(t *testing.T) {
	state := NewUpdateProgressState()
	if ok := state.Begin("2.0.0"); !ok {
		t.Fatal("Begin should acquire lock")
	}

	state.Publish(70)
	state.Publish(60) // should remain monotonic
	state.Finish(errors.New("download failed"))

	failed := state.Snapshot()
	if failed.InProgress {
		t.Fatal("failed snapshot should not be in progress")
	}
	if failed.Percent != 70 {
		t.Fatalf("failed snapshot percent = %d, want 70", failed.Percent)
	}
	if failed.Error == "" {
		t.Fatal("failed snapshot should include error")
	}

	if ok := state.Begin("2.0.1"); !ok {
		t.Fatal("Begin should succeed after failed run")
	}
	state.Publish(80)
	state.Finish(nil)

	success := state.Snapshot()
	if success.InProgress {
		t.Fatal("success snapshot should not be in progress")
	}
	if success.Percent != 100 {
		t.Fatalf("success snapshot percent = %d, want 100", success.Percent)
	}
	if success.Error != "" {
		t.Fatalf("success snapshot error = %q, want empty", success.Error)
	}
	if success.Version != "2.0.1" {
		t.Fatalf("success snapshot version = %q, want %q", success.Version, "2.0.1")
	}
}
