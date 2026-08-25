# QuantumBPM Go SDK

Official Go SDK for the [QuantumBPM](https://quantumbpm.com) platform - DMN evaluation, BPMN process orchestration, and external job workers.

## Installation

```bash
go get github.com/QuantumBPM/quantum-go-sdk
```

Requires Go 1.23+.

## What's in the box

| Package                            | Purpose                                                                       |
| ---------------------------------- | ----------------------------------------------------------------------------- |
| `quantumbpm` (root)                | Top-level `Client` exposing `.DMN` and `.BPMN`, plus `NewWorker(...)`         |
| `quantumbpm/auth`                  | `TokenProvider` interface, `ZitadelTokenProvider`, `StaticTokenProvider`      |
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

    "github.com/google/uuid"

    quantumbpm "github.com/QuantumBPM/quantum-go-sdk"
    "github.com/QuantumBPM/quantum-go-sdk/auth"
    "github.com/QuantumBPM/quantum-go-sdk/variables"
)

func main() {
    provider, err := auth.NewZitadelTokenProvider(
        "./service-account.json",       // Zitadel JSON Key file
        "https://auth.quantumbpm.com",  // issuer
        "your-zitadel-project-id",      // audience scope
    )
    if err != nil {
        log.Fatal(err)
    }

    client, err := quantumbpm.New(quantumbpm.Config{
        BaseURL:       "https://api.quantumbpm.com",
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

### Zitadel service account

```go
provider, err := auth.NewZitadelTokenProvider(
    "./service-account.json",      // path to JSON Key file
    "https://auth.quantumbpm.com", // issuer URL
    "your-zitadel-project-id",     // adds the audience scope
)
```

The provider caches tokens in-memory until shortly before expiry.

### Static bearer token

For Enterprise deployments that issue long-lived API keys, or in tests where a token is acquired out of band:

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

### Evaluate by platform UUID

When you already hold a database-version pointer:

```go
result, err := client.DMN.EvaluateByID(ctx, definitionUUID, vars)
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

// Start an instance from the deployed process definition.
workflowID, err := client.BPMN.StartInstance(ctx, draft.ProcessDefinition.Id, variables.New().
    Set("applicantID", "u-123").
    Set("requestedAmt", 25000))
```

### Inspect runtime state

```go
state, err := client.BPMN.GetInstance(ctx, workflowID)
fmt.Println(state.Status, state.ActiveScopes)

vars, err := client.BPMN.GetInstanceVariables(ctx, workflowID)

children, err := client.BPMN.GetInstanceChildren(ctx, workflowID)
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

ctx, cancel := context.WithCancel(context.Background())
defer cancel()
if err := worker.Run(ctx); err != nil {
    log.Fatal(err)
}
```

`Run` blocks until `ctx` is cancelled. In-flight handlers are allowed to finish before it returns.

### Concurrency, polling, and locks

```go
worker.Handle("send-email", handler,
    workers.WithMaxJobs(10),                    // up to 10 in flight per task type
    workers.WithPollTimeout(45*time.Second),    // long-poll wait
    workers.WithLockDuration(2*time.Minute),    // exclusive lock per job
)
```

Concurrency is per task type. Different task types run in independent goroutine pools. The runtime auto-renews the lock at half the lock-duration interval while the handler runs.

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
resp, err := raw.MigrateBpmnInstanceWithResponse(ctx, projectID, workflowID, body)
```

## License

MIT License - see [LICENSE](LICENSE) for details.
