package workers

import (
	"bytes"
	"log"
	"strings"
	"testing"
	"unicode/utf8"
)

func newTestWorker(limit int) (*Worker, *bytes.Buffer) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	if limit <= 0 {
		limit = defaultMaxErrorMessageBytes
	}
	return &Worker{
		logger:               logger,
		maxErrorMessageBytes: limit,
	}, &buf
}

func TestClampWorkerErrorMessage_PassThroughWhenSmall(t *testing.T) {
	w, logs := newTestWorker(0)
	msg := "boom"
	got := w.clampWorkerErrorMessage("payment", msg)
	if got != msg {
		t.Fatalf("expected pass-through, got %q", got)
	}
	if logs.Len() != 0 {
		t.Fatalf("expected no log, got %q", logs.String())
	}
}

func TestClampWorkerErrorMessage_TruncatesAtDefault(t *testing.T) {
	w, logs := newTestWorker(0)
	msg := strings.Repeat("x", 100_000)
	got := w.clampWorkerErrorMessage("payment", msg)
	if len(got) > defaultMaxErrorMessageBytes {
		t.Fatalf("clamped len=%d exceeds limit=%d", len(got), defaultMaxErrorMessageBytes)
	}
	if !strings.HasSuffix(got, " bytes]") {
		t.Fatalf("expected truncation marker suffix, got tail=%q", got[len(got)-40:])
	}
	if !strings.Contains(logs.String(), "WORKER_ERROR message truncated") {
		t.Fatalf("expected WARN log, got %q", logs.String())
	}
	if strings.Count(logs.String(), "\n") != 1 {
		t.Fatalf("expected exactly one log line, got %q", logs.String())
	}
}

func TestClampWorkerErrorMessage_HonorsOverride(t *testing.T) {
	w, _ := newTestWorker(256)
	got := w.clampWorkerErrorMessage("payment", strings.Repeat("x", 10_000))
	if len(got) > 256 {
		t.Fatalf("clamped len=%d exceeds override=256", len(got))
	}
}

func TestClampWorkerErrorMessage_CutsOnRuneBoundary(t *testing.T) {
	// "é" is 2 bytes in UTF-8. Build a message whose limit boundary would
	// otherwise split a rune.
	w, _ := newTestWorker(200)
	msg := strings.Repeat("é", 1000) // 2000 bytes
	got := w.clampWorkerErrorMessage("t", msg)
	if !utf8.ValidString(got) {
		t.Fatalf("clamped string contains invalid UTF-8")
	}
}
