# QuantumBPM Go SDK

Official Go SDK for the [QuantumBPM](https://quantumbpm.com) platform - DMN evaluation, BPMN process orchestration, and external job workers.

## Installation

```bash
go get github.com/QuantumBPM/quantum-go-sdk/v2
```

Requires Go 1.25+.

## What's in the box

| Package                            | Purpose                                                                       |
| ---------------------------------- | ----------------------------------------------------------------------------- |
| `quantumbpm` (root)                | Top-level `Client` exposing `.DMN` and `.BPMN`, plus `NewWorker(...)`         |
| `quantumbpm/auth`                  | `TokenProvider`, `ClientCredentialsTokenProvider`, `StaticTokenProvider`      |
| `quantumbpm/dmn`                   | DMN evaluation: stored definitions, ad-hoc XML, batch                         |
| `quantumbpm/bpmn`                  | BPMN resources, instances, messaging, user tasks, processes                   |
| `quantumbpm/workers`               | External job worker runtime - long-poll, lock heartbeat, dispatch             |
| `quantumbpm/variables`             | `Vars` map type with typed accessors and FEEL-context conversion              |
| `quantumbpm/generated`             | OpenAPI-generated client. Reachable via `Client.Raw()`, never hand-edited     |

## Quick start

```go
package main

import (
    "context"
    "log"
    "os"

    "github.com/google/uuid"

    quantumbpm "github.com/QuantumBPM/quantum-go-sdk/v2"
    "github.com/QuantumBPM/quantum-go-sdk/v2/auth"
    "github.com/QuantumBPM/quantum-go-sdk/v2/variables"
)

func main() {
    provider := auth.NewClientCredentialsTokenProvider(auth.ClientCredentials{
        TokenURL:     "https://idp.example.com/realms/acme/protocol/openid-connect/token",
        ClientID:     "billing-svc",
        ClientSecret: os.Getenv("QUANTUMBPM_CLIENT_SECRET"),
    })

    client, err := quantumbpm.New(quantumbpm.Config{
        BaseURL:       "https://quantumbpm.example.com",
        ProjectID:     uuid.MustParse("00000000-0000-0000-0000-000000000000"),
        TokenProvider: provider,
    })
    if err != nil {
        log.Fatal(err)
    }

    ctx := context.Background()
    result, err := client.DMN.Evaluate(ctx, "loan-eligibility", variables.New().
        Set("requestedAmt", 1000).
        Set("creditScore", 720))
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("decisions: %+v", result)
}
```

## Authentication

The `auth.TokenProvider` interface returns a bearer token on each request. Two implementations ship out of the box.

### OAuth 2.0 client credentials

`auth.ClientCredentialsTokenProvider` fetches access tokens from your OIDC identity provider (Keycloak, Microsoft Entra ID, Okta, Auth0 and others) with the client credentials grant. Register a confidential client for your service at the identity provider and pass its token endpoint and credentials:

```go
provider := auth.NewClientCredentialsTokenProvider(auth.ClientCredentials{
    TokenURL:     "https://idp.example.com/realms/acme/protocol/openid-connect/token",
    ClientID:     "billing-svc",
    ClientSecret: os.Getenv("QUANTUMBPM_CLIENT_SECRET"),
    Scopes:       []string{"openid"}, // optional
    Audience:     "quantumbpm",       // optional, some providers such as Auth0 require it
    HTTPClient:   httpClient,         // optional, defaults to http.DefaultClient
})
```

The exact token endpoint depends on the identity provider. The provider's OIDC discovery document (`/.well-known/openid-configuration`) lists it as `token_endpoint`.

`Scopes` are sent space-joined as the `scope` parameter and `Audience` as the `audience` parameter. Each is left out of the request when empty. The client must be set up at the identity provider so its tokens carry the audience and roles your QuantumBPM deployment expects.

The provider caches the token and fetches a new one once less than 60 seconds of its lifetime remain. It is safe for concurrent use. A non-2xx response from the token endpoint returns an error with the HTTP status and response body.

### Static bearer token

For a token acquired out of band, for example by another library or in tests:

```go
provider := auth.NewStaticTokenProvider("eyJhbGciOi...")
```

### Bring your own

Implement the interface:

```go
type TokenProvider interface {
    Token(ctx context.Context) (string, error)
}
```

`auth.TokenProviderFunc(fn)` adapts a plain function literal.

## DMN evaluation

The `client.DMN` sub-client offers four methods.

### Evaluate a stored definition

```go
result, err := client.DMN.Evaluate(ctx, "loan-eligibility", variables.New().
    Set("requestedAmt", 5000).
    Set("creditScore", 720))
```

Returns `map[string]EvaluationResult` keyed by decision ID. Each result has `Value`, `HitRules`, `Error`, and `Type`.

Pin a version, restrict the evaluated decisions, or attach decision services:

```go
result, err := client.DMN.Evaluate(ctx, "loan-eligibility", vars,
    dmn.WithVersion(3),
    dmn.WithDecisions("eligibility", "rate"),
)
```

### Ad-hoc XML evaluation

For "evaluate while editing" flows that don't store the XML:

```go
result, err := client.DMN.EvaluateDesign(ctx, dmnXML, vars,
    dmn.WithAdditionalXMLs(importedXML1, importedXML2),
    dmn.WithDesignDecisions("eligibility"),
)
```

### Batch ad-hoc evaluation

```go
rows := []variables.Vars{
    variables.New().Set("requestedAmt", 1000),
    variables.New().Set("requestedAmt", 5000),
    variables.New().Set("requestedAmt", 25000),
}
batch, err := client.DMN.EvaluateDesignBatch(ctx, dmnXML, rows)
```

## BPMN processes

`client.BPMN` covers the full BPMN runtime surface. Highlights:

### Deploy and start

```go
// Stage a BPMN draft, then deploy it.
draft, err := client.BPMN.CreateResource(ctx, "loan-process", bpmnXML)
if err != nil { log.Fatal(err) }

if err := client.BPMN.DeployResource(ctx, draft.Id); err != nil {
    log.Fatal(err)
}

// Start an instance of the deployed process's latest version, by its BPMN
// process ID. bpmn.WithStartVersion(n) starts a specific version.
instanceID, err := client.BPMN.StartInstance(ctx, "loan-process", variables.New().
    Set("applicantID", "u-123").
    Set("requestedAmt", 25000))
```

### Inspect runtime state

```go
state, err := client.BPMN.GetInstance(ctx, instanceID)
fmt.Println(state.Status, state.ActiveScopes)

vars, err := client.BPMN.GetInstanceVariables(ctx, instanceID)

children, err := client.BPMN.GetInstanceChildren(ctx, instanceID)

// History pages run oldest first. Without options a page holds the newest
// entries, and an entry read by its seq carries its inputs and outputs.
page, err := client.BPMN.GetInstanceHistory(ctx, instanceID, bpmn.WithHistoryFromSeq(1), bpmn.WithHistoryLimit(100))
entry, err := client.BPMN.GetInstanceHistoryEntry(ctx, instanceID, *page.Items[0].Seq)

version, err := client.BPMN.GetProcessVersion(ctx, "loan-process", 2)
```

### Send messages and signals

```go
err := client.BPMN.PublishMessage(ctx, "loan-approved",
    variables.New().Set("approvedAmt", 24000),
    bpmn.WithCorrelationKeys(bpmn.CorrelationKeys{ /* ... */ }),
    bpmn.WithMessageTTL("PT5M"),
)

err = client.BPMN.PublishSignal(ctx, "system-maintenance", variables.New())
```

### User tasks

```go
page, err := client.BPMN.ListUserTasks(ctx,
    bpmn.WithUserTaskAssignee("alice@example.com"),
    bpmn.WithUserTaskStatus("CREATED"),
)

// Lists of instances and user tasks page by cursor: pass the page's
// NextCursor back for the next one. It is nil on the last page.
if page.Pagination.NextCursor != nil {
    page, err = client.BPMN.ListUserTasks(ctx,
        bpmn.WithUserTaskAssignee("alice@example.com"),
        bpmn.WithUserTaskStatus("CREATED"),
        bpmn.WithUserTaskCursor(*page.Pagination.NextCursor),
    )
}

// The caller's tasks soonest due first, and the project's overdue ones.
mine, err := client.BPMN.ListUserTasksForCaller(ctx, bpmn.WithUserTaskSort("dueDate"))
overdue, err := client.BPMN.ListUserTasks(ctx,
    bpmn.WithUserTaskStatus("CREATED"),
    bpmn.WithUserTaskDueBefore(time.Now()),
)

err = client.BPMN.CompleteUserTask(ctx, executionKey,
    variables.New().Set("approved", true))

// Or fail with a BPMN error code (matches boundary error events):
err = client.BPMN.ThrowUserTaskError(ctx, executionKey, "REVIEW_REJECTED", variables.New())
```

## External job workers

Workers handle service tasks asynchronously. Register a handler per `taskType`, then call `Run`. The runtime owns long-polling, lock heartbeats, dispatch, and outcome mapping.

### Minimal worker

```go
worker := client.NewWorker(workers.Config{ClientID: "billing-svc"})

worker.Handle("send-email", func(ctx context.Context, j *workers.Job) (variables.Vars, error) {
    to, _ := variables.Get[string](j.Vars, "recipient")
    subject, _ := variables.Get[string](j.Vars, "subject")

    if err := emailer.Send(to, subject); err != nil {
        return nil, err  // → ThrowError("WORKER_ERROR"); retry budget decrements
    }
    return variables.New().Set("messageID", "msg-123"), nil  // → Complete
})

// Cancel ctx on Ctrl+C and on SIGTERM (sent by Kubernetes and Docker on stop).
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()
if err := worker.Run(ctx); err != nil {
    log.Fatal(err)
}
```

`Run` blocks until `ctx` is cancelled, then shuts down gracefully (see [Locks, heartbeats, and shutdown](#locks-heartbeats-and-shutdown)).

### Concurrency, polling, and locks

```go
worker.Handle("send-email", handler,
    workers.WithMaxJobs(10),                    // up to 10 in flight per task type
    workers.WithPollTimeout(45*time.Second),    // long-poll wait
    workers.WithLockDuration(2*time.Minute),    // exclusive lock per job
)
```

Concurrency is per task type. Different task types run in independent goroutine pools. The server clamps the lock duration to between 10s and 5m by default (`ENGINE_WORKER_LOCK_MIN`, `ENGINE_WORKER_LOCK_MAX`), so `WithLockDuration(11*time.Minute)` gets a 5m lock. While the handler runs, the runtime heartbeats at half of the lock the server granted.

### Locks, heartbeats, and shutdown

A polled job is locked to the worker's `ClientID`. Each heartbeat extends the lock by the clamped lock duration, and the server caps one attempt at 30 minutes by default (`ENGINE_JOB_MAX_HOLD`). If the lock runs out (missed heartbeats, the cap, or a crashed worker), the server charges the job a retry and offers it again, or raises an incident when no retries are left. So abandoning a running job always costs a retry.

When a heartbeat answers that the worker no longer holds the job, the runtime stops heartbeating it, cancels the handler's `ctx` and drops its result. Handlers doing long work should watch `ctx.Done()`.

Cancelling the `ctx` passed to `Run` starts a graceful shutdown:

1. Polling stops at once, including a long-poll in progress. The worker only ever polls for free slots, so it holds no job it is not running.
2. Running handlers keep going. Their `ctx` stays alive, heartbeats continue, and their results are reported.
3. After the shutdown timeout (default 20s), handlers still running are cancelled and not reported, and `Run` returns. Their locks run out on the server, which costs each of them a retry.

```go
worker := client.NewWorker(workers.Config{
    ClientID:        "billing-svc",
    ShutdownTimeout: 45 * time.Second,
})
```

On Kubernetes, set the pod's `terminationGracePeriodSeconds` above the shutdown timeout, so the pod is not killed before `Run` returns. The Kubernetes default of 30s covers the 20s default.

### Throwing typed BPMN errors

Return a `*workers.BpmnError` to fail the job with a code that boundary error events on the originating service task can catch:

```go
worker.Handle("charge-card", func(ctx context.Context, j *workers.Job) (variables.Vars, error) {
    if err := charge(j.Vars); errors.Is(err, ErrInsufficientFunds) {
        return nil, workers.NewBpmnError("INSUFFICIENT_FUNDS",
            variables.New().Set("availableBalance", 12.00))
    }
    return variables.New().Set("transactionID", txID), nil
})
```

Other errors (transient I/O, panics) are reported as `WORKER_ERROR`, which the server treats as a retryable failure that decrements the job's retry budget.

### Typed handlers

`HandleTyped[T]` decodes the job's input variables into a struct before invoking the handler:

```go
type EmailJob struct {
    Recipient string `json:"recipient"`
    Subject   string `json:"subject"`
}

workers.HandleTyped(worker, "send-email",
    func(ctx context.Context, j *workers.Job, in EmailJob) (variables.Vars, error) {
        if err := emailer.Send(in.Recipient, in.Subject); err != nil {
            return nil, err
        }
        return variables.New().Set("messageID", "msg-123"), nil
    },
)
```

## Variables

The `variables.Vars` type is a thin `map[string]any` shared by DMN, BPMN, and workers.

### Construction

```go
v := variables.New().Set("amount", 100).Set("name", "Alice")
v := variables.From(map[string]any{"amount": 100, "name": "Alice"})
```

### Typed access

```go
amt, err := variables.Get[float64](v, "amount")
flag, err := variables.Get[bool](v, "approved")

type Loan struct {
    RequestedAmt float64 `json:"requestedAmt"`
    Approved     bool    `json:"approved"`
}
loan, err := variables.As[Loan](v)
```

`Get` and `As` use a JSON round-trip, so nested structs and arbitrary value types work without custom registration.

## Escape hatch

The `client.Raw()` and `client.BPMN.Raw()` methods expose the underlying generated client for endpoints that are not wrapped (instance migration, modification, ad-hoc triggers, batch job complete/error, etc.).

```go
raw := client.Raw()
resp, err := raw.MigrateBpmnInstanceWithResponse(ctx, projectID, instanceID, body)
```

## License

MIT License - see [LICENSE](LICENSE) for details.
