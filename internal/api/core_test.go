package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/s-ui/s-ui/internal/config"
	"github.com/s-ui/s-ui/internal/core"
)

type coreErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail"`
}

type coreStatusPayload struct {
	Running   bool            `json:"running"`
	State     string          `json:"state"`
	Actions   []string        `json:"actions"`
	LastError json.RawMessage `json:"lastError"`
}

type coreLogsPayload struct {
	Path    string   `json:"path"`
	Count   int      `json:"count"`
	Entries []string `json:"entries"`
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder, v interface{}) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decode JSON: %v; body=%s", err, rec.Body.String())
	}
}

func hasAction(actions []string, target string) bool {
	for _, a := range actions {
		if a == target {
			return true
		}
	}
	return false
}

func testCoreConfig(t *testing.T, binaryPath string) *config.Config {
	t.Helper()
	dataDir := t.TempDir()
	configPath := filepath.Join(dataDir, "sing-box.json")
	if err := os.WriteFile(configPath, []byte(`{"log":{}}`), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if binaryPath == "" {
		binaryPath = filepath.Join(dataDir, "bin", "sing-box")
	}
	return &config.Config{
		DataDir:           dataDir,
		SingboxConfigPath: configPath,
		SingboxBinaryPath: binaryPath,
	}
}

func overrideUpdateDependencies(t *testing.T, state *core.UpdateProgressState, runner func(cfg *config.Config, onProgress func(percent int)) error) {
	t.Helper()
	oldStateProvider := updateProgressStateProvider
	oldRunner := coreUpdateRunner
	updateProgressStateProvider = func() *core.UpdateProgressState { return state }
	coreUpdateRunner = runner
	t.Cleanup(func() {
		updateProgressStateProvider = oldStateProvider
		coreUpdateRunner = oldRunner
	})
}

func TestStatusHandlerReturnsNotInstalledState(t *testing.T) {
	cfg := testCoreConfig(t, "")
	req := httptest.NewRequest(http.MethodGet, "/api/core/status", nil)
	rec := httptest.NewRecorder()

	StatusHandler(nil, cfg).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("StatusHandler status = %d, want %d", rec.Code, http.StatusOK)
	}

	var got coreStatusPayload
	decodeJSON(t, rec, &got)
	if got.State != string(core.CoreStateNotInstalled) {
		t.Fatalf("state = %q, want %q", got.State, core.CoreStateNotInstalled)
	}
	if !hasAction(got.Actions, "install") {
		t.Fatalf("actions = %v, want install action", got.Actions)
	}
}

func TestStartHandlerReturnsCoreNotInstalled(t *testing.T) {
	cfg := testCoreConfig(t, filepath.Join(t.TempDir(), "missing-sing-box"))
	req := httptest.NewRequest(http.MethodPost, "/api/core/start", nil)
	rec := httptest.NewRecorder()

	StartHandler(nil, cfg).ServeHTTP(rec, req)
	if rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("StartHandler status = %d, want 4xx", rec.Code)
	}

	var got coreErrorPayload
	decodeJSON(t, rec, &got)
	if got.Code != string(core.ProcessErrorNotInstalled) {
		t.Fatalf("error code = %q, want %q", got.Code, core.ProcessErrorNotInstalled)
	}
}

func TestRestartHandlerReturnsCoreNotInstalled(t *testing.T) {
	pm := core.NewProcessManagerWithBinary("", "")
	if pm.IsRunning() {
		t.Skip("host already has sing-box running; skip restart not_installed assertion")
	}

	cfg := testCoreConfig(t, filepath.Join(t.TempDir(), "missing-sing-box"))
	req := httptest.NewRequest(http.MethodPost, "/api/core/restart", nil)
	rec := httptest.NewRecorder()

	RestartHandler(nil, cfg).ServeHTTP(rec, req)
	if rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("RestartHandler status = %d, want 4xx", rec.Code)
	}

	var got coreErrorPayload
	decodeJSON(t, rec, &got)
	if got.Code != string(core.ProcessErrorNotInstalled) {
		t.Fatalf("error code = %q, want %q", got.Code, core.ProcessErrorNotInstalled)
	}
}

func TestStopHandlerReturnsAlreadyStopped(t *testing.T) {
	pm := core.NewProcessManagerWithBinary("", "")
	if pm.IsRunning() {
		t.Skip("host already has sing-box running; skip already_stopped assertion")
	}

	cfg := testCoreConfig(t, filepath.Join(t.TempDir(), "missing-sing-box"))
	req := httptest.NewRequest(http.MethodPost, "/api/core/stop", nil)
	rec := httptest.NewRecorder()

	StopHandler(nil, cfg).ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("StopHandler status = %d, want %d", rec.Code, http.StatusConflict)
	}

	var got coreErrorPayload
	decodeJSON(t, rec, &got)
	if got.Code != string(core.ProcessErrorAlreadyStopped) {
		t.Fatalf("error code = %q, want %q", got.Code, core.ProcessErrorAlreadyStopped)
	}
}

func TestStartFailureSetsErrorStateActions(t *testing.T) {
	pm := core.NewProcessManagerWithBinary("", "")
	if pm.IsRunning() {
		t.Skip("host already has sing-box running; skip error-state startup test")
	}

	dataDir := t.TempDir()
	failBinary := filepath.Join(dataDir, "fail-sing-box")
	if err := os.WriteFile(failBinary, []byte("#!/bin/sh\nexit 1\n"), 0755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	cfg := testCoreConfig(t, failBinary)
	startReq := httptest.NewRequest(http.MethodPost, "/api/core/start", nil)
	startRec := httptest.NewRecorder()
	StartHandler(nil, cfg).ServeHTTP(startRec, startReq)
	if startRec.Code != http.StatusInternalServerError {
		t.Fatalf("StartHandler status = %d, want %d", startRec.Code, http.StatusInternalServerError)
	}

	var startErr coreErrorPayload
	decodeJSON(t, startRec, &startErr)
	if startErr.Code != string(core.ProcessErrorStartFailed) {
		t.Fatalf("error code = %q, want %q", startErr.Code, core.ProcessErrorStartFailed)
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/api/core/status", nil)
	statusRec := httptest.NewRecorder()
	StatusHandler(nil, cfg).ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("StatusHandler status = %d, want %d", statusRec.Code, http.StatusOK)
	}

	var statusBody coreStatusPayload
	decodeJSON(t, statusRec, &statusBody)
	if statusBody.State != string(core.CoreStateError) {
		t.Fatalf("state = %q, want %q", statusBody.State, core.CoreStateError)
	}
	if !hasAction(statusBody.Actions, "retry_start") || !hasAction(statusBody.Actions, "view_logs") {
		t.Fatalf("actions = %v, want retry_start and view_logs", statusBody.Actions)
	}
	if len(statusBody.LastError) == 0 || string(statusBody.LastError) == "null" {
		t.Fatalf("lastError = %s, want non-null context", string(statusBody.LastError))
	}
}

func TestLogsHandlerReadsTailLines(t *testing.T) {
	cfg := testCoreConfig(t, filepath.Join(t.TempDir(), "missing-sing-box"))
	logPath := filepath.Join(cfg.DataDir, "sing-box.log")
	if err := os.WriteFile(logPath, []byte("line-1\nline-2\nline-3\n"), 0644); err != nil {
		t.Fatalf("write log file: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/core/logs?lines=2", nil)
	rec := httptest.NewRecorder()
	LogsHandler(nil, cfg).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("LogsHandler status = %d, want %d", rec.Code, http.StatusOK)
	}

	var got coreLogsPayload
	decodeJSON(t, rec, &got)
	if got.Count != 2 {
		t.Fatalf("count = %d, want 2", got.Count)
	}
	if len(got.Entries) != 2 || got.Entries[0] != "line-2" || got.Entries[1] != "line-3" {
		t.Fatalf("entries = %v, want [line-2 line-3]", got.Entries)
	}
}

func TestLogsHandlerReturnsNotFoundWhenMissing(t *testing.T) {
	cfg := testCoreConfig(t, filepath.Join(t.TempDir(), "missing-sing-box"))
	req := httptest.NewRequest(http.MethodGet, "/api/core/logs", nil)
	rec := httptest.NewRecorder()

	LogsHandler(nil, cfg).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("LogsHandler status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	var got coreErrorPayload
	decodeJSON(t, rec, &got)
	if got.Code != "CORE_LOG_NOT_FOUND" {
		t.Fatalf("error code = %q, want %q", got.Code, "CORE_LOG_NOT_FOUND")
	}
}

func TestUpdateHandlerReturnsConflictWhenUpdateAlreadyInProgress(t *testing.T) {
	cfg := testCoreConfig(t, filepath.Join(t.TempDir(), "missing-sing-box"))
	state := core.NewUpdateProgressState()
	started := make(chan struct{})
	release := make(chan struct{})
	var startedOnce sync.Once
	var callCount atomic.Int32

	overrideUpdateDependencies(t, state, func(cfg *config.Config, onProgress func(percent int)) error {
		callCount.Add(1)
		startedOnce.Do(func() { close(started) })
		<-release
		if onProgress != nil {
			onProgress(100)
		}
		return nil
	})

	req1 := httptest.NewRequest(http.MethodPost, "/api/core/update", nil)
	rec1 := httptest.NewRecorder()
	UpdateHandler(nil, cfg).ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusAccepted {
		t.Fatalf("first UpdateHandler status = %d, want %d", rec1.Code, http.StatusAccepted)
	}

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first update runner to start")
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/core/update", nil)
	rec2 := httptest.NewRecorder()
	UpdateHandler(nil, cfg).ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusConflict {
		t.Fatalf("second UpdateHandler status = %d, want %d", rec2.Code, http.StatusConflict)
	}
	var conflictBody coreErrorPayload
	decodeJSON(t, rec2, &conflictBody)
	if conflictBody.Code != "CORE_UPDATE_CONFLICT" {
		t.Fatalf("conflict code = %q, want %q", conflictBody.Code, "CORE_UPDATE_CONFLICT")
	}
	if got := callCount.Load(); got != 1 {
		t.Fatalf("update runner called %d times, want 1", got)
	}

	close(release)
	deadline := time.Now().Add(time.Second)
	for state.Snapshot().InProgress {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for update state to finish")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestUpdateStreamHandlerSendsSnapshotWithRequiredHeaders(t *testing.T) {
	cfg := testCoreConfig(t, filepath.Join(t.TempDir(), "missing-sing-box"))
	state := core.NewUpdateProgressState()
	if ok := state.Begin("1.2.3"); !ok {
		t.Fatal("Begin should acquire lock")
	}
	state.Publish(42)
	defer state.Finish(nil)

	overrideUpdateDependencies(t, state, func(cfg *config.Config, onProgress func(percent int)) error {
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/core/update/stream", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		UpdateStreamHandler(nil, cfg).ServeHTTP(rec, req)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("stream handler did not exit after context cancellation")
	}

	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want %q", got, "text/event-stream")
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache, no-transform" {
		t.Fatalf("Cache-Control = %q, want %q", got, "no-cache, no-transform")
	}
	if got := rec.Header().Get("X-Accel-Buffering"); got != "no" {
		t.Fatalf("X-Accel-Buffering = %q, want %q", got, "no")
	}

	body := rec.Body.String()
	if !strings.Contains(body, "data: ") {
		t.Fatalf("stream body missing SSE event prefix: %s", body)
	}
	if !strings.Contains(body, "\"inProgress\":true") {
		t.Fatalf("stream body missing inProgress snapshot: %s", body)
	}
	if !strings.Contains(body, "\"percent\":42") {
		t.Fatalf("stream body missing percent snapshot: %s", body)
	}
}
