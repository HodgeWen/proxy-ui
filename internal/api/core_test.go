package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

type fakeCoreUpdater struct {
	run func(func(int)) error
}

func (f fakeCoreUpdater) UpdateWithProgress(onProgress func(int)) error {
	if f.run == nil {
		return nil
	}
	return f.run(onProgress)
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

func resetUpdateProgressState(t *testing.T) {
	t.Helper()
	state := core.GlobalUpdateProgressState()
	if ok := state.Begin("reset"); ok {
		state.Finish(nil)
		return
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if ok := state.Begin("reset"); ok {
			state.Finish(nil)
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("update progress lock still held")
}

func waitUpdateIdle(t *testing.T) {
	t.Helper()
	state := core.GlobalUpdateProgressState()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if !state.Snapshot().InProgress {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("update progress still in progress after timeout")
}

func TestUpdateHandlerAcceptedThenConflict(t *testing.T) {
	resetUpdateProgressState(t)
	cfg := testCoreConfig(t, filepath.Join(t.TempDir(), "fake-sing-box"))

	started := make(chan struct{})
	release := make(chan struct{})

	previousFactory := newCoreUpdaterForRequest
	newCoreUpdaterForRequest = func(cfg *config.Config) coreUpdater {
		return fakeCoreUpdater{
			run: func(onProgress func(int)) error {
				close(started)
				if onProgress != nil {
					onProgress(25)
				}
				<-release
				if onProgress != nil {
					onProgress(100)
				}
				return nil
			},
		}
	}
	defer func() {
		newCoreUpdaterForRequest = previousFactory
	}()

	firstReq := httptest.NewRequest(http.MethodPost, "/api/core/update", nil)
	firstRec := httptest.NewRecorder()
	UpdateHandler(nil, cfg).ServeHTTP(firstRec, firstReq)
	if firstRec.Code != http.StatusAccepted {
		t.Fatalf("first update status = %d, want %d", firstRec.Code, http.StatusAccepted)
	}

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("first update did not start async updater")
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/api/core/update", nil)
	secondRec := httptest.NewRecorder()
	UpdateHandler(nil, cfg).ServeHTTP(secondRec, secondReq)
	if secondRec.Code != http.StatusConflict {
		t.Fatalf("second update status = %d, want %d", secondRec.Code, http.StatusConflict)
	}

	var conflict coreErrorPayload
	decodeJSON(t, secondRec, &conflict)
	if conflict.Code != "CORE_UPDATE_CONFLICT" {
		t.Fatalf("conflict code = %q, want %q", conflict.Code, "CORE_UPDATE_CONFLICT")
	}

	close(release)
	waitUpdateIdle(t)
}

func TestUpdateStreamHandlerSendsSnapshotAndHeaders(t *testing.T) {
	resetUpdateProgressState(t)
	state := core.GlobalUpdateProgressState()
	if ok := state.Begin("v-test"); !ok {
		t.Fatal("failed to begin update for stream test")
	}
	defer func() {
		if state.Snapshot().InProgress {
			state.Finish(nil)
		}
	}()
	state.Publish(42)

	cfg := testCoreConfig(t, filepath.Join(t.TempDir(), "fake-sing-box"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/core/update/stream", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		UpdateStreamHandler(nil, cfg).ServeHTTP(rec, req)
		close(done)
	}()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(rec.Body.String(), "data: ") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "data: ") {
		t.Fatalf("expected SSE data event, got body: %q", body)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("content-type = %q, want %q", got, "text/event-stream")
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache, no-transform" {
		t.Fatalf("cache-control = %q, want %q", got, "no-cache, no-transform")
	}
	if got := rec.Header().Get("X-Accel-Buffering"); got != "no" {
		t.Fatalf("x-accel-buffering = %q, want %q", got, "no")
	}

	firstLine := strings.Split(strings.TrimSpace(body), "\n")[0]
	if !strings.HasPrefix(firstLine, "data: ") {
		t.Fatalf("first line = %q, want SSE data line", firstLine)
	}
	var snapshot core.UpdateProgressSnapshot
	if err := json.Unmarshal([]byte(strings.TrimPrefix(firstLine, "data: ")), &snapshot); err != nil {
		t.Fatalf("unmarshal first SSE snapshot: %v", err)
	}
	if !snapshot.InProgress || snapshot.Percent != 42 {
		t.Fatalf("first SSE snapshot = %+v, want in-progress percent=42", snapshot)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("stream handler did not exit after context cancel")
	}
}
