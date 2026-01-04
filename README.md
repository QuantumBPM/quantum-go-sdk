# QuantumDMN Go SDK

Official Go SDK for the [QuantumDMN](https://quantumdmn.com) DMN Engine API.

## Installation

```bash
go get github.com/QuantumDMN/dmn-go-sdk
```

## Features

- **High-Level `EngineClient`**: Simplified interaction for evaluating decisions.
- **Full API Coverage**: Access to all underlying API endpoints (Projects, Definitions, Executions).
- **Authentication**: Flexible `TokenProvider` interface for integration with Zitadel or other auth providers.
- **Type-Safe**: Generated structures for DMN models and API requests.

## Quick Start

### 1. Initialize Client

The `EngineClient` requires a base URL, a Project ID, and a `TokenProvider`.

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/QuantumDMN/dmn-go-sdk/pkg/quantumdmn"
)

func main() {
    ctx := context.Background()

    // Define a TokenProvider
    // This function is called for every request to get a valid Bearer token.
    tokenProvider := func(ctx context.Context) (string, error) {
        // Implement your token retrieval logic here (e.g., OIDC client credentials flow)
        return "your-access-token", nil
    }

    // Create the Engine Client
    client, err := quantumdmn.NewEngineClient(
        "https://api.quantumdmn.com", 
        "your-project-id-uuid", 
        tokenProvider,
    )
    if err != nil {
        log.Fatal(err)
    }

    // ... use client
}
```

### 2. Evaluate a Decision

Use the `Evaluate` method to execute a decision by its XML Definition ID (Business Key).

```go
    // Define input variables (Context)
    inputs := map[string]interface{}{
        "age":    25,
        "income": 50000,
        "riskFactors": []string{"debt", "history"},
    }

    // Evaluate
    // Arguments: Context, XML ID, Version (nil for latest), Inputs
    results, err := client.Evaluate(ctx, "risk-scoring", nil, inputs)
    if err != nil {
        log.Fatal(err)
    }

    // Process Results
    for name, res := range results {
        fmt.Printf("Decision '%s': %v\n", name, res.Value)
    }
```

## Advanced Usage

### Accessing Underlying API

If you need to access other API endpoints (e.g., managing projects or definitions), you can use the generated client directly, though `EngineClient` focuses on evaluation.

To obtain the raw generated client, you can construct it via `NewAuthenticatedClient`:

```go
client, err := quantumdmn.NewAuthenticatedClient("https://api.quantumdmn.com", tokenProvider)
// client.ListProjectsWithResponse(ctx)
```

## Authentication

The SDK uses a `TokenProvider` function signature: `func(ctx context.Context) (string, error)`.

### Authenticating with Zitadel Service User (Built-in)

The SDK provides a `NewZitadelTokenProvider` helper to authenticate using a JSON Key file.

```go
// Create provider from key file
// Note: projectID is the Zitadel Project ID (different from DMN Project ID) 
// used to request project-specific audience and roles.
tp, err := quantumdmn.NewZitadelTokenProvider(
    "./service-account.json",      // Path to JSON Key
    "https://auth.quantumdmn.com", // Issuer URL
    "zitadel-project-id",          // Zitadel Project ID (required for accessing granted projects)
)
if err != nil {
    log.Fatal(err)
}

// Initialize client
client, err := quantumdmn.NewEngineClient(
    "https://api.quantumdmn.com", 
    "your-project-id", // DMN Project ID (different from Zitadel Project ID)
    tp,
)
```

## License

MIT License - see [LICENSE](LICENSE) for details.
