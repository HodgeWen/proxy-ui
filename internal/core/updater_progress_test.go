package core

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestUpdateDownloadProgressWithContentLength(t *testing.T) {
	payload := bytes.Repeat([]byte("a"), 256*1024)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "262144")
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "release.tar.gz")
	var events []int
	err := downloadFileWithProgress(srv.URL, dest, func(percent int) {
		events = append(events, percent)
	})
	if err != nil {
		t.Fatalf("downloadFileWithProgress() error = %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected progress callbacks with content-length")
	}
	if events[len(events)-1] != 100 {
		t.Fatalf("final percent = %d, want 100", events[len(events)-1])
	}
	for i := 1; i < len(events); i++ {
		if events[i] < events[i-1] {
			t.Fatalf("progress not monotonic: %v", events)
		}
	}

	info, statErr := os.Stat(dest)
	if statErr != nil {
		t.Fatalf("downloaded file stat error = %v", statErr)
	}
	if info.Size() != int64(len(payload)) {
		t.Fatalf("downloaded file size = %d, want %d", info.Size(), len(payload))
	}
}

func TestUpdateDownloadProgressWithoutContentLengthFallsBackToCompletion(t *testing.T) {
	payload := bytes.Repeat([]byte("b"), 64*1024)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Del("Content-Length")
		flusher, _ := w.(http.Flusher)
		for i := 0; i < 4; i++ {
			start := i * 16384
			end := start + 16384
			_, _ = w.Write(payload[start:end])
			if flusher != nil {
				flusher.Flush()
			}
		}
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "release.tar.gz")
	var events []int
	err := downloadFileWithProgress(srv.URL, dest, func(percent int) {
		events = append(events, percent)
	})
	if err != nil {
		t.Fatalf("downloadFileWithProgress() error = %v", err)
	}
	if len(events) != 1 || events[0] != 100 {
		t.Fatalf("events = %v, want [100]", events)
	}
}
