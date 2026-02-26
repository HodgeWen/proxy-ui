package core

import (
	"sync"
	"time"
)

// CoreState represents the current lifecycle state of sing-box.
type CoreState string

const (
	CoreStateNotInstalled CoreState = "not_installed"
	CoreStateStopped      CoreState = "stopped"
	CoreStateRunning      CoreState = "running"
	CoreStateError        CoreState = "error"
)

// LastFailureContext stores the latest startup failure details for API display.
type LastFailureContext struct {
	Message    string    `json:"message"`
	OccurredAt time.Time `json:"occurredAt"`
	Stage      string    `json:"stage"`
	Source     string    `json:"source"`
}

// LifecycleSnapshot is a serializable state view for API responses.
type LifecycleSnapshot struct {
	State     CoreState           `json:"state"`
	Running   bool                `json:"running"`
	Installed bool                `json:"installed"`
	Actions   []string            `json:"actions"`
	LastError *LastFailureContext `json:"lastError,omitempty"`
}

var (
	lastFailureMu sync.RWMutex
	lastFailure   *LastFailureContext
)

// ResolveCoreState returns the lifecycle snapshot using fixed priority:
// not_installed > running > error > stopped.
func ResolveCoreState(pm *ProcessManager) LifecycleSnapshot {
	installed := pm.Available()
	running := pm.IsRunning()
	failure := getLastFailure()

	state := resolveState(installed, running, failure)
	snapshot := LifecycleSnapshot{
		State:     state,
		Running:   running,
		Installed: installed,
		Actions:   ActionMatrix(state),
	}
	if state == CoreStateError {
		snapshot.LastError = failure
	}

	return snapshot
}

// ActionMatrix returns allowed actions for the given state.
func ActionMatrix(state CoreState) []string {
	switch state {
	case CoreStateRunning:
		return []string{"stop", "restart"}
	case CoreStateStopped:
		return []string{"start"}
	case CoreStateNotInstalled:
		return []string{"install"}
	case CoreStateError:
		return []string{"retry_start", "view_logs"}
	default:
		return []string{}
	}
}

func resolveState(installed bool, running bool, failure *LastFailureContext) CoreState {
	switch {
	case !installed:
		return CoreStateNotInstalled
	case running:
		return CoreStateRunning
	case failure != nil:
		return CoreStateError
	default:
		return CoreStateStopped
	}
}

func newFailureContext(message, stage, source string) *LastFailureContext {
	return &LastFailureContext{
		Message:    message,
		OccurredAt: time.Now().UTC(),
		Stage:      stage,
		Source:     source,
	}
}

func setLastFailure(ctx *LastFailureContext) {
	lastFailureMu.Lock()
	defer lastFailureMu.Unlock()
	lastFailure = ctx
}

func clearLastFailure() {
	lastFailureMu.Lock()
	defer lastFailureMu.Unlock()
	lastFailure = nil
}

func getLastFailure() *LastFailureContext {
	lastFailureMu.RLock()
	defer lastFailureMu.RUnlock()

	if lastFailure == nil {
		return nil
	}
	cp := *lastFailure
	return &cp
}
