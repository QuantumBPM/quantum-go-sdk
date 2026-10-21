package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/QuantumBPM/quantum-go-sdk/generated"
	"github.com/QuantumBPM/quantum-go-sdk/variables"
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

func TestHeartbeatInterval(t *testing.T) {
	now := time.Now()
	at := func(d time.Duration) *time.Time {
		ts := now.Add(d)
		return &ts
	}
	cases := []struct {
		name      string
		requested time.Duration
		expires   *time.Time
		want      time.Duration
	}{
		{"server clamped the lock", 11 * time.Minute, at(5 * time.Minute), 150 * time.Second},
		{"granted as requested", 30 * time.Second, at(30 * time.Second), 15 * time.Second},
		{"granted above requested", 4 * time.Second, at(10 * time.Second), 2 * time.Second},
		{"no lockExpiresAt", 11 * time.Minute, nil, 330 * time.Second},
		{"lockExpiresAt already passed", time.Minute, at(-time.Second), 30 * time.Second},
		{"at least 1s", time.Second, at(time.Second), time.Second},
	}
	for _, c := range cases {
		if got := heartbeatInterval(c.requested, c.expires, now); got != c.want {
			t.Errorf("%s: interval = %s, want %s", c.name, got, c.want)
		}
	}
}

func TestNew_BlankClientIDFallsBack(t *testing.T) {
	host, _ := os.Hostname()
	want := fmt.Sprintf("worker-%s-%d", host, os.Getpid())
	for _, id := range []string{"", "   ", "\t\n"} {
		if got := New(nil, openapi_types.UUID{}, Config{ClientID: id}).clientID; got != want {
			t.Errorf("clientID for %q = %q, want %q", id, got, want)
		}
	}
	if got := New(nil, openapi_types.UUID{}, Config{ClientID: " svc "}).clientID; got != "svc" {
		t.Errorf("clientID = %q, want %q", got, "svc")
	}
}

// syncBuffer is a log sink that is safe to read while the worker writes.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// fakeAPI serves the external job endpoints. A poll takes up to maxJobs
// queued jobs and otherwise blocks until a job is queued or the client goes away.
type fakeAPI struct {
	jobs            chan generated.ExternalJob
	heartbeatStatus atomic.Int32
	completeStatus  atomic.Int32
	logs            syncBuffer

	mu         sync.Mutex
	pollMax    []int
	heartbeats int
	completed  []string
}

func newFakeAPI(t *testing.T, cfg Config) (*Worker, *fakeAPI) {
	t.Helper()
	f := &fakeAPI{jobs: make(chan generated.ExternalJob, 16)}
	f.heartbeatStatus.Store(http.StatusOK)
	f.completeStatus.Store(http.StatusOK)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /projects/{pid}/bpmn/external-jobs/poll", f.poll)
	mux.HandleFunc("POST /projects/{pid}/bpmn/external-jobs/{key}/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.heartbeats++
		f.mu.Unlock()
		w.WriteHeader(int(f.heartbeatStatus.Load()))
	})
	mux.HandleFunc("POST /projects/{pid}/bpmn/external-jobs/{key}/complete", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.completed = append(f.completed, r.PathValue("key"))
		f.mu.Unlock()
		status := int(f.completeStatus.Load())
		w.WriteHeader(status)
		switch status {
		case http.StatusConflict:
			_, _ = w.Write([]byte(`{"code":"CONFLICT","message":"the job was canceled"}`))
		case http.StatusNotFound:
			_, _ = w.Write([]byte(`{"code":"NOT_FOUND","message":"external job not found"}`))
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	api, err := generated.NewClientWithResponses(srv.URL)
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	cfg.Logger = log.New(&f.logs, "", 0)
	return New(api, openapi_types.UUID{}, cfg), f
}

func (f *fakeAPI) poll(w http.ResponseWriter, r *http.Request) {
	var req generated.PollBpmnJobRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	maxJobs := 1
	if req.MaxJobs != nil {
		maxJobs = *req.MaxJobs
	}
	f.mu.Lock()
	f.pollMax = append(f.pollMax, maxJobs)
	f.mu.Unlock()

	var out []generated.ExternalJob
	select {
	case j := <-f.jobs:
		out = append(out, j)
	case <-r.Context().Done():
		w.WriteHeader(http.StatusNoContent)
		return
	}
	for len(out) < maxJobs && len(f.jobs) > 0 {
		out = append(out, <-f.jobs)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (f *fakeAPI) pollMaxes() []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int(nil), f.pollMax...)
}

func (f *fakeAPI) heartbeatCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.heartbeats
}

func (f *fakeAPI) completedKeys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.completed...)
}

func testJob(key string) generated.ExternalJob {
	return generated.ExternalJob{ExecutionKey: key, WorkflowID: "wf-" + key, TaskType: "t", NodeID: "n"}
}

func startRun(t *testing.T, w *Worker) (context.CancelFunc, <-chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	return cancel, done
}

func waitRun(t *testing.T, done <-chan error, within time.Duration) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(within):
		t.Fatalf("Run did not return within %s", within)
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func waitClosed(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

func TestRun_HeartbeatConflictCancelsHandler(t *testing.T) {
	w, f := newFakeAPI(t, Config{})
	f.heartbeatStatus.Store(http.StatusConflict)
	f.jobs <- testJob("job-1")

	cancelled := make(chan struct{})
	w.Handle("t", func(ctx context.Context, _ *Job) (variables.Vars, error) {
		select {
		case <-ctx.Done():
			close(cancelled)
		case <-time.After(5 * time.Second):
		}
		return variables.New().Set("ok", true), nil
	}, WithLockDuration(time.Second))

	cancel, done := startRun(t, w)
	waitClosed(t, cancelled, "handler ctx cancelled by heartbeat 409")
	cancel()
	waitRun(t, done, 5*time.Second)

	if got := f.completedKeys(); len(got) != 0 {
		t.Fatalf("complete sent after lost lock: %v", got)
	}
	if n := f.heartbeatCount(); n != 1 {
		t.Fatalf("heartbeats = %d, want 1 (stop after lost lock)", n)
	}
	if n := strings.Count(f.logs.String(), "this worker no longer holds the job, handler cancelled"); n != 1 {
		t.Fatalf("expected one lock lost log, got %d in %q", n, f.logs.String())
	}
}

func TestRun_HeartbeatsAtHalfOfGrantedLock(t *testing.T) {
	w, f := newFakeAPI(t, Config{})
	job := testJob("job-1")
	expires := time.Now().Add(2 * time.Second)
	job.LockExpiresAt = &expires
	f.jobs <- job

	release := make(chan struct{})
	w.Handle("t", func(context.Context, *Job) (variables.Vars, error) {
		<-release
		return variables.New(), nil
	}, WithLockDuration(time.Hour))

	cancel, done := startRun(t, w)
	waitFor(t, "heartbeat within the granted 2s lock", func() bool { return f.heartbeatCount() > 0 })
	close(release)
	cancel()
	waitRun(t, done, 5*time.Second)
}

func TestRun_CompleteConflictLogsLockLost(t *testing.T) {
	w, f := newFakeAPI(t, Config{})
	f.completeStatus.Store(http.StatusConflict)
	f.jobs <- testJob("job-1")
	w.Handle("t", func(context.Context, *Job) (variables.Vars, error) {
		return variables.New(), nil
	})

	cancel, done := startRun(t, w)
	waitFor(t, "complete call", func() bool { return len(f.completedKeys()) == 1 })
	cancel()
	waitRun(t, done, 5*time.Second)

	if !strings.Contains(f.logs.String(), "WARN complete job-1: result discarded: the job was canceled") {
		t.Fatalf("expected the server's reason in the log, got %q", f.logs.String())
	}
}

func TestRun_CompleteNotFoundLogsReason(t *testing.T) {
	w, f := newFakeAPI(t, Config{})
	f.completeStatus.Store(http.StatusNotFound)
	f.jobs <- testJob("job-1")
	w.Handle("t", func(context.Context, *Job) (variables.Vars, error) {
		return variables.New(), nil
	})

	cancel, done := startRun(t, w)
	waitFor(t, "complete call", func() bool { return len(f.completedKeys()) == 1 })
	cancel()
	waitRun(t, done, 5*time.Second)

	if !strings.Contains(f.logs.String(), "WARN complete job-1: result discarded: external job not found") {
		t.Fatalf("expected the server's reason in the log, got %q", f.logs.String())
	}
}

func TestRun_ShutdownLetsRunningHandlerReport(t *testing.T) {
	w, f := newFakeAPI(t, Config{})
	f.jobs <- testJob("job-1")

	started := make(chan struct{})
	release := make(chan struct{})
	var handlerCtxErr error
	w.Handle("t", func(ctx context.Context, _ *Job) (variables.Vars, error) {
		close(started)
		<-release
		handlerCtxErr = ctx.Err()
		return variables.New().Set("ok", true), nil
	}, WithLockDuration(time.Second))

	cancel, done := startRun(t, w)
	waitClosed(t, started, "handler start")
	cancel()
	hb := f.heartbeatCount()
	waitFor(t, "heartbeat during shutdown", func() bool { return f.heartbeatCount() > hb })
	close(release)
	waitRun(t, done, 5*time.Second)

	if handlerCtxErr != nil {
		t.Fatalf("handler ctx cancelled by shutdown: %v", handlerCtxErr)
	}
	if got := f.completedKeys(); len(got) != 1 || got[0] != "job-1" {
		t.Fatalf("completed = %v, want [job-1]", got)
	}
	if logs := f.logs.String(); strings.Contains(logs, "complete") {
		t.Fatalf("complete failed: %q", logs)
	}
}

func TestRun_CancelWithAllSlotsBusyReturns(t *testing.T) {
	w, f := newFakeAPI(t, Config{})
	f.jobs <- testJob("job-1")
	f.jobs <- testJob("job-2")

	var running atomic.Int32
	release := make(chan struct{})
	w.Handle("t", func(context.Context, *Job) (variables.Vars, error) {
		running.Add(1)
		<-release
		return variables.New(), nil
	}, WithMaxJobs(2))

	cancel, done := startRun(t, w)
	waitFor(t, "both handlers running", func() bool { return running.Load() == 2 })
	cancel()
	close(release)
	waitRun(t, done, 5*time.Second)

	if got := f.completedKeys(); len(got) != 2 {
		t.Fatalf("completed = %v, want 2 jobs", got)
	}
}

func TestRun_CancelStopsLongPoll(t *testing.T) {
	w, f := newFakeAPI(t, Config{})
	w.Handle("t", func(context.Context, *Job) (variables.Vars, error) {
		return variables.New(), nil
	})

	cancel, done := startRun(t, w)
	waitFor(t, "long poll", func() bool { return len(f.pollMaxes()) == 1 })
	cancel()
	waitRun(t, done, time.Second)
}

func TestRun_ShutdownTimeoutCancelsHandler(t *testing.T) {
	w, f := newFakeAPI(t, Config{ShutdownTimeout: 200 * time.Millisecond})
	f.jobs <- testJob("job-1")

	started := make(chan struct{})
	cancelled := make(chan struct{})
	w.Handle("t", func(ctx context.Context, _ *Job) (variables.Vars, error) {
		close(started)
		select {
		case <-ctx.Done():
			close(cancelled)
		case <-time.After(5 * time.Second):
		}
		return variables.New(), nil
	})

	cancel, done := startRun(t, w)
	waitClosed(t, started, "handler start")
	cancel()
	waitRun(t, done, 5*time.Second)
	waitClosed(t, cancelled, "handler ctx cancelled by shutdown timeout")

	// The result would be reported right after the handler returns.
	time.Sleep(100 * time.Millisecond)
	if got := f.completedKeys(); len(got) != 0 {
		t.Fatalf("complete sent after shutdown timeout: %v", got)
	}
	if !strings.Contains(f.logs.String(), "abandoned after 200ms") {
		t.Fatalf("expected shutdown timeout log, got %q", f.logs.String())
	}
}

func TestRun_PollsOnlyFreeSlots(t *testing.T) {
	w, f := newFakeAPI(t, Config{})
	f.jobs <- testJob("job-1")
	f.jobs <- testJob("job-2")

	var running atomic.Int32
	release := make(chan struct{})
	w.Handle("t", func(context.Context, *Job) (variables.Vars, error) {
		running.Add(1)
		<-release
		return variables.New(), nil
	}, WithMaxJobs(3))

	cancel, done := startRun(t, w)
	// The first poll takes both queued jobs, so the second asks for one.
	waitFor(t, "second poll", func() bool { return len(f.pollMaxes()) == 2 })
	f.jobs <- testJob("job-3")
	waitFor(t, "three handlers running", func() bool { return running.Load() == 3 })

	// All slots are busy, so no poll runs until one handler finishes.
	time.Sleep(100 * time.Millisecond)
	if got := f.pollMaxes(); len(got) != 2 {
		t.Fatalf("polled while all slots were busy: %v", got)
	}
	release <- struct{}{}
	waitFor(t, "third poll", func() bool { return len(f.pollMaxes()) == 3 })

	cancel()
	close(release)
	waitRun(t, done, 5*time.Second)

	if got, want := f.pollMaxes(), []int{3, 1, 1}; !slices.Equal(got, want) {
		t.Fatalf("poll maxJobs = %v, want %v", got, want)
	}
}
