// Package workers provides a long-poll runtime for BPMN external service
// tasks. Register a handler per task type, then call Run; the runtime owns
// polling, lock heartbeats, dispatch, and outcome mapping (Complete on
// success, ThrowError on a BpmnError, ThrowError with a generic code on
// any other handler error).
package workers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
	"unicode/utf8"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/QuantumBPM/quantum-go-sdk/generated"
	"github.com/QuantumBPM/quantum-go-sdk/variables"
)

// Defaults tuned to match the server-side defaults documented in the API.
const (
	defaultMaxJobs              = 1
	defaultPollTimeout          = 30 * time.Second
	defaultLockDuration         = 30 * time.Second
	defaultMaxErrorMessageBytes = 2048
	heartbeatRatio              = 2 // heartbeat at lockDuration/heartbeatRatio
	pollErrorBackoff            = 2 * time.Second
)

// Job is the work unit handed to a Handler. It wraps the generated
// ExternalJob and exposes the variables decoded into a Vars value.
type Job struct {
	*generated.ExternalJob
	// Vars holds the input variables resolved by the service task. Use
	// variables.Get[T] or variables.As[T] to decode into typed values.
	Vars variables.Vars
}

// Handler processes a single job. Return value semantics:
//   - (vars, nil)              → Complete with vars merged into instance
//   - (_, *BpmnError)          → ThrowError with the supplied code
//   - (_, any other error)     → ThrowError with a generic code; retry budget decrements
type Handler func(ctx context.Context, job *Job) (variables.Vars, error)

// HandleOption tunes per-task-type registration.
type HandleOption func(*handleOpts)

type handleOpts struct {
	maxJobs      int
	pollTimeout  time.Duration
	lockDuration time.Duration
}

// WithMaxJobs caps how many jobs the runtime acquires per poll for the
// task type. Higher values amortize the round-trip; lower values share
// the queue with other workers.
func WithMaxJobs(n int) HandleOption {
	return func(o *handleOpts) { o.maxJobs = n }
}

// WithPollTimeout sets how long each long-poll call waits before returning
// 204. Defaults to 30s.
func WithPollTimeout(d time.Duration) HandleOption {
	return func(o *handleOpts) { o.pollTimeout = d }
}

// WithLockDuration sets the exclusive lock duration on each acquired job.
// The runtime renews the lock automatically while the handler runs.
func WithLockDuration(d time.Duration) HandleOption {
	return func(o *handleOpts) { o.lockDuration = d }
}

type registration struct {
	taskType string
	handler  Handler
	opts     handleOpts
}

// Worker is a long-poll runtime owning a set of handlers, one per task type.
// Use Worker.Handle to register a handler; Worker.Run starts the polling
// goroutines and blocks until ctx is cancelled.
type Worker struct {
	api                  *generated.ClientWithResponses
	projectID            openapi_types.UUID
	clientID             string
	logger               *log.Logger
	maxErrorMessageBytes int

	mu           sync.Mutex
	registrations map[string]*registration
}

// Config configures a Worker.
type Config struct {
	// ClientID is the worker's stable identity. Used to attribute job locks
	// and counted as one of the active workers per task type. If empty,
	// derived from the host name.
	ClientID string
	// Logger receives lifecycle messages (poll errors, fatal handler errors).
	// Nil disables logging.
	Logger *log.Logger
	// MaxErrorMessageBytes caps the byte length of the auto-built
	// WORKER_ERROR message attached when a handler returns a non-BpmnError.
	// Zero falls back to 2048. User-thrown BpmnError variables are not clamped.
	MaxErrorMessageBytes int
}

// New constructs a Worker bound to projectID. api should be an authenticated
// client (typically obtained from auth.NewClient).
func New(api *generated.ClientWithResponses, projectID openapi_types.UUID, cfg Config) *Worker {
	clientID := cfg.ClientID
	if clientID == "" {
		host, _ := os.Hostname()
		clientID = fmt.Sprintf("worker-%s-%d", host, os.Getpid())
	}
	logger := cfg.Logger
	if logger == nil {
		logger = log.New(os.Stderr, "[quantumbpm-worker] ", log.LstdFlags)
	}
	maxErrMsg := cfg.MaxErrorMessageBytes
	if maxErrMsg <= 0 {
		maxErrMsg = defaultMaxErrorMessageBytes
	}
	return &Worker{
		api:                  api,
		projectID:            projectID,
		clientID:             clientID,
		logger:               logger,
		maxErrorMessageBytes: maxErrMsg,
		registrations:        make(map[string]*registration),
	}
}

// Handle registers handler as the processor for taskType. Re-registering an
// existing taskType replaces the previous handler.
func (w *Worker) Handle(taskType string, handler Handler, opts ...HandleOption) {
	o := handleOpts{
		maxJobs:      defaultMaxJobs,
		pollTimeout:  defaultPollTimeout,
		lockDuration: defaultLockDuration,
	}
	for _, opt := range opts {
		opt(&o)
	}
	w.mu.Lock()
	w.registrations[taskType] = &registration{taskType: taskType, handler: handler, opts: o}
	w.mu.Unlock()
}

// HandleTyped registers a handler that decodes the job's input variables
// into a value of type T before invoking handler. Use it for opt-in typed
// dispatch when the producing service task has a known variable schema.
func HandleTyped[T any](w *Worker, taskType string, handler func(ctx context.Context, job *Job, in T) (variables.Vars, error), opts ...HandleOption) {
	w.Handle(taskType, func(ctx context.Context, job *Job) (variables.Vars, error) {
		var in T
		if len(job.Vars) > 0 {
			b, err := json.Marshal(job.Vars)
			if err != nil {
				return nil, fmt.Errorf("workers: marshal vars: %w", err)
			}
			if err := json.Unmarshal(b, &in); err != nil {
				return nil, fmt.Errorf("workers: decode vars into %T: %w", in, err)
			}
		}
		return handler(ctx, job, in)
	}, opts...)
}

// Run starts the polling loops and blocks until ctx is cancelled. Each
// registered task type is polled in its own goroutine; jobs are dispatched
// concurrently per task type up to maxJobs.
//
// Returns nil when ctx is cancelled (graceful shutdown after in-flight jobs
// finish), or an error if Run is invoked with no registered handlers.
func (w *Worker) Run(ctx context.Context) error {
	w.mu.Lock()
	regs := make([]*registration, 0, len(w.registrations))
	for _, r := range w.registrations {
		regs = append(regs, r)
	}
	w.mu.Unlock()

	if len(regs) == 0 {
		return errors.New("workers: no handlers registered")
	}

	var wg sync.WaitGroup
	for _, r := range regs {
		wg.Add(1)
		go func(r *registration) {
			defer wg.Done()
			w.runTaskType(ctx, r)
		}(r)
	}
	wg.Wait()
	return nil
}

// runTaskType is the per-task-type long-poll loop.
func (w *Worker) runTaskType(ctx context.Context, r *registration) {
	sem := make(chan struct{}, r.opts.maxJobs)
	var inflight sync.WaitGroup

	for {
		if ctx.Err() != nil {
			break
		}

		// Bound the next poll to maxJobs - currently in-flight. Acquire one
		// slot before polling so we never carry more leases than the cap.
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			break
		}

		// Use a poll timeout slightly larger than the configured one so the
		// HTTP context doesn't fire before the server's 204.
		pollCtx, cancel := context.WithTimeout(ctx, r.opts.pollTimeout+10*time.Second)
		jobs, err := w.poll(pollCtx, r)
		cancel()

		if err != nil {
			<-sem
			if ctx.Err() != nil {
				break
			}
			w.logger.Printf("poll %s: %v", r.taskType, err)
			select {
			case <-time.After(pollErrorBackoff):
			case <-ctx.Done():
			}
			continue
		}

		if len(jobs) == 0 {
			<-sem
			continue
		}

		// First job uses the slot already acquired; remaining jobs each
		// acquire their own.
		for i, job := range jobs {
			if i > 0 {
				select {
				case sem <- struct{}{}:
				case <-ctx.Done():
					return
				}
			}
			inflight.Add(1)
			go func(j generated.ExternalJob) {
				defer inflight.Done()
				defer func() { <-sem }()
				w.dispatch(ctx, r, &j)
			}(job)
		}
	}

	inflight.Wait()
}

// poll executes one long-poll. Returns nil, nil on 204.
func (w *Worker) poll(ctx context.Context, r *registration) ([]generated.ExternalJob, error) {
	timeoutStr := r.opts.pollTimeout.String()
	lockStr := r.opts.lockDuration.String()
	maxJobs := r.opts.maxJobs

	resp, err := w.api.PollBpmnExternalJobsWithResponse(ctx, w.projectID, generated.PollBpmnExternalJobsJSONRequestBody{
		ClientID:     w.clientID,
		TaskType:     r.taskType,
		LockDuration: &lockStr,
		Timeout:      &timeoutStr,
		MaxJobs:      &maxJobs,
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() == 204 || resp.JSON200 == nil {
		return nil, nil
	}
	return *resp.JSON200, nil
}

// dispatch runs the handler for one job, sending Heartbeats and finalizing
// with Complete or ThrowError.
func (w *Worker) dispatch(parent context.Context, r *registration, job *generated.ExternalJob) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	// Heartbeat goroutine — refresh the lock at lockDuration/heartbeatRatio.
	go w.heartbeat(ctx, r, job)

	wrapped := &Job{
		ExternalJob: job,
		Vars:        variables.FromWireMap(job.Variables),
	}

	resultVars, err := w.safeRun(ctx, r.handler, wrapped)

	// Stop heartbeating before finalizing so we don't race with the terminal call.
	cancel()

	finalCtx, finalCancel := context.WithTimeout(parent, 15*time.Second)
	defer finalCancel()

	switch {
	case err == nil:
		w.complete(finalCtx, job, resultVars)
	case isBpmnError(err):
		var be *BpmnError
		_ = errors.As(err, &be)
		w.throwError(finalCtx, job, be.Code, be.Variables)
	default:
		w.logger.Printf("handler %s: %v", r.taskType, err)
		msg := w.clampWorkerErrorMessage(r.taskType, err.Error())
		w.throwError(finalCtx, job, "WORKER_ERROR", variables.New().Set("error", msg))
	}
}

// clampWorkerErrorMessage shortens an unhandled handler exception's message
// to the configured byte budget. UTF-8 safe (cuts on rune boundary). Logs a
// WARN and appends a truncation marker when it triggers.
func (w *Worker) clampWorkerErrorMessage(taskType, msg string) string {
	limit := w.maxErrorMessageBytes
	if limit <= 0 || len(msg) <= limit {
		return msg
	}
	marker := fmt.Sprintf("…[truncated, original %d bytes]", len(msg))
	budget := limit - len(marker)
	if budget < 0 {
		budget = 0
	}
	// Cut on a rune boundary so we never emit a half-codepoint.
	cut := budget
	for cut > 0 && !utf8.RuneStart(msg[cut]) {
		cut--
	}
	w.logger.Printf("workers: WORKER_ERROR message truncated for task=%s from %d to %d bytes", taskType, len(msg), limit)
	return msg[:cut] + marker
}

// safeRun invokes handler and recovers panics into errors.
func (w *Worker) safeRun(ctx context.Context, h Handler, job *Job) (out variables.Vars, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("workers: handler panic: %v", r)
		}
	}()
	return h(ctx, job)
}

// heartbeat refreshes the job lock until ctx is cancelled.
func (w *Worker) heartbeat(ctx context.Context, r *registration, job *generated.ExternalJob) {
	interval := r.opts.lockDuration / heartbeatRatio
	if interval < time.Second {
		interval = time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	lockStr := r.opts.lockDuration.String()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			hbCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_, err := w.api.HeartbeatBpmnExternalJobWithResponse(hbCtx, w.projectID, job.ExecutionKey, generated.HeartbeatBpmnExternalJobJSONRequestBody{
				ClientID:     w.clientID,
				LockDuration: &lockStr,
			})
			cancel()
			if err != nil {
				w.logger.Printf("heartbeat %s: %v", job.ExecutionKey, err)
			}
		}
	}
}

func (w *Worker) complete(ctx context.Context, job *generated.ExternalJob, vars variables.Vars) {
	resp, err := w.api.CompleteBpmnExternalJobWithResponse(ctx, w.projectID, job.ExecutionKey, generated.CompleteBpmnExternalJobJSONRequestBody{
		WorkflowID: job.WorkflowID,
		Variables:  vars.ToWireMap(),
	})
	if err != nil {
		w.logger.Printf("complete %s: %v", job.ExecutionKey, err)
		return
	}
	if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
		w.logger.Printf("complete %s: status %d", job.ExecutionKey, resp.StatusCode())
	}
}

func (w *Worker) throwError(ctx context.Context, job *generated.ExternalJob, code string, vars variables.Vars) {
	resp, err := w.api.ThrowBpmnExternalJobErrorWithResponse(ctx, w.projectID, job.ExecutionKey, generated.ThrowBpmnExternalJobErrorJSONRequestBody{
		ErrorCode: code,
		Variables: vars.ToWireMap(),
	})
	if err != nil {
		w.logger.Printf("throw-error %s: %v", job.ExecutionKey, err)
		return
	}
	if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
		w.logger.Printf("throw-error %s: status %d", job.ExecutionKey, resp.StatusCode())
	}
}

func isBpmnError(err error) bool {
	var be *BpmnError
	return errors.As(err, &be)
}
