package core

import (
	"sync"
	"time"
)

// UpdateProgressSnapshot is the single source of truth for core update progress.
type UpdateProgressSnapshot struct {
	InProgress bool      `json:"inProgress"`
	Percent    int       `json:"percent"`
	Version    string    `json:"version"`
	Error      string    `json:"error,omitempty"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// UpdateProgressState coordinates one update workflow and broadcasts snapshots.
type UpdateProgressState struct {
	updateMu sync.Mutex

	mu          sync.RWMutex
	snapshot    UpdateProgressSnapshot
	subscribers map[int]chan UpdateProgressSnapshot
	nextID      int
}

var globalUpdateProgressState = NewUpdateProgressState()

// GlobalUpdateProgressState returns the shared update progress coordinator.
func GlobalUpdateProgressState() *UpdateProgressState {
	return globalUpdateProgressState
}

// NewUpdateProgressState creates a new in-memory update progress coordinator.
func NewUpdateProgressState() *UpdateProgressState {
	return &UpdateProgressState{
		snapshot: UpdateProgressSnapshot{
			InProgress: false,
			Percent:    0,
			UpdatedAt:  time.Now().UTC(),
		},
		subscribers: make(map[int]chan UpdateProgressSnapshot),
	}
}

// Begin enters update mode with mutex TryLock semantics.
func (s *UpdateProgressState) Begin(version string) bool {
	if !s.updateMu.TryLock() {
		return false
	}

	s.mu.Lock()
	s.snapshot.InProgress = true
	s.snapshot.Version = version
	s.snapshot.Percent = 0
	s.snapshot.Error = ""
	s.snapshot.UpdatedAt = time.Now().UTC()
	snapshot := s.snapshot
	subs := s.collectSubscribersLocked()
	s.mu.Unlock()

	s.broadcast(subs, snapshot)
	return true
}

// Publish pushes latest download progress while preserving monotonic percent.
func (s *UpdateProgressState) Publish(percent int) {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	s.mu.Lock()
	if percent < s.snapshot.Percent {
		percent = s.snapshot.Percent
	}
	s.snapshot.Percent = percent
	s.snapshot.UpdatedAt = time.Now().UTC()
	snapshot := s.snapshot
	subs := s.collectSubscribersLocked()
	s.mu.Unlock()

	s.broadcast(subs, snapshot)
}

// Finish closes an update run and releases the mutex lock.
func (s *UpdateProgressState) Finish(err error) {
	s.mu.Lock()
	s.snapshot.InProgress = false
	if err == nil {
		s.snapshot.Percent = 100
		s.snapshot.Error = ""
	} else {
		s.snapshot.Error = err.Error()
	}
	s.snapshot.UpdatedAt = time.Now().UTC()
	snapshot := s.snapshot
	subs := s.collectSubscribersLocked()
	s.mu.Unlock()

	s.broadcast(subs, snapshot)
	s.updateMu.Unlock()
}

// Snapshot returns a consistent copy of current update state.
func (s *UpdateProgressState) Snapshot() UpdateProgressSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot
}

// Subscribe registers a subscriber channel and pushes current snapshot first.
func (s *UpdateProgressState) Subscribe() (int, <-chan UpdateProgressSnapshot) {
	s.mu.Lock()
	id := s.nextID
	s.nextID++

	ch := make(chan UpdateProgressSnapshot, 1)
	s.subscribers[id] = ch
	snapshot := s.snapshot
	s.mu.Unlock()

	s.sendLatest(ch, snapshot)
	return id, ch
}

// Unsubscribe removes and closes a subscriber channel.
func (s *UpdateProgressState) Unsubscribe(id int) {
	s.mu.Lock()
	ch, ok := s.subscribers[id]
	if ok {
		delete(s.subscribers, id)
	}
	s.mu.Unlock()

	if ok {
		close(ch)
	}
}

func (s *UpdateProgressState) collectSubscribersLocked() []chan UpdateProgressSnapshot {
	subs := make([]chan UpdateProgressSnapshot, 0, len(s.subscribers))
	for _, ch := range s.subscribers {
		subs = append(subs, ch)
	}
	return subs
}

func (s *UpdateProgressState) broadcast(subs []chan UpdateProgressSnapshot, snapshot UpdateProgressSnapshot) {
	for _, ch := range subs {
		s.sendLatest(ch, snapshot)
	}
}

func (s *UpdateProgressState) sendLatest(ch chan UpdateProgressSnapshot, snapshot UpdateProgressSnapshot) {
	select {
	case ch <- snapshot:
		return
	default:
	}

	select {
	case <-ch:
	default:
	}

	select {
	case ch <- snapshot:
	default:
	}
}
