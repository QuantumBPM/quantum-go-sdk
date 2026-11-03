// Package quantumbpm is the Go SDK for QuantumBPM: BPMN 2.0 workflow
// orchestration, DMN 1.5 decision evaluation with FEEL, and external job workers.
//
// Construct a Client with quantumbpm.New, then reach the per-domain
// sub-clients via .DMN and .BPMN. To run external job workers, call
// Client.NewWorker.
//
//	provider := auth.NewClientCredentialsTokenProvider(auth.ClientCredentials{TokenURL: tokenURL, ClientID: id, ClientSecret: secret})
//	client, _ := quantumbpm.New(quantumbpm.Config{
//	    BaseURL:       "https://quantumbpm.example.com",
//	    ProjectID:     projectUUID,
//	    TokenProvider: provider,
//	})
//
//	result, _ := client.DMN.Evaluate(ctx, "loan-eligibility", vars)
//	instanceID, _ := client.BPMN.StartInstance(ctx, "order-flow", vars)
//
//	worker := client.NewWorker(workers.Config{ClientID: "billing-svc"})
//	worker.Handle("send-email", func(ctx context.Context, j *workers.Job) (variables.Vars, error) {
//	    // ... send email ...
//	    return variables.New().Set("messageID", id), nil
//	})
//	worker.Run(ctx)
package quantumbpm

import (
	"errors"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/QuantumBPM/quantum-go-sdk/v2/auth"
	"github.com/QuantumBPM/quantum-go-sdk/v2/bpmn"
	"github.com/QuantumBPM/quantum-go-sdk/v2/dmn"
	"github.com/QuantumBPM/quantum-go-sdk/v2/generated"
	"github.com/QuantumBPM/quantum-go-sdk/v2/workers"
)

// Config holds construction parameters for Client.
type Config struct {
	// BaseURL is the QuantumBPM API root (e.g. https://quantumbpm.example.com).
	BaseURL string

	// ProjectID is the workspace the client is scoped to. All DMN, BPMN, and
	// worker calls operate against this project.
	ProjectID uuid.UUID

	// TokenProvider supplies a bearer token on every request. Use
	// auth.NewClientCredentialsTokenProvider for tokens from an OIDC provider
	// or auth.NewStaticTokenProvider for a fixed token.
	TokenProvider auth.TokenProvider
}

// Client is the top-level QuantumBPM SDK entry point.
type Client struct {
	api       *generated.ClientWithResponses
	projectID openapi_types.UUID

	// DMN evaluates DMN definitions in the project.
	DMN *dmn.Client
	// BPMN drives BPMN process resources, instances, messaging, and user tasks.
	BPMN *bpmn.Client
}

// New constructs a Client. Returns an error when required config is missing
// or the underlying HTTP client cannot be initialized.
func New(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, errors.New("quantumbpm: BaseURL is required")
	}
	if cfg.TokenProvider == nil {
		return nil, errors.New("quantumbpm: TokenProvider is required")
	}

	api, err := auth.NewClient(cfg.BaseURL, cfg.TokenProvider)
	if err != nil {
		return nil, err
	}

	pid := openapi_types.UUID(cfg.ProjectID)
	return &Client{
		api:       api,
		projectID: pid,
		DMN:       dmn.New(api, pid),
		BPMN:      bpmn.New(api, pid),
	}, nil
}

// NewWorker constructs an external job worker bound to this client's
// project. Register handlers via Worker.Handle (or workers.HandleTyped[T])
// and call Worker.Run to start polling.
func (c *Client) NewWorker(cfg workers.Config) *workers.Worker {
	return workers.New(c.api, c.projectID, cfg)
}

// Raw exposes the generated OpenAPI client for endpoints not covered by the
// wrapper (migrate, modify, ad-hoc, batch jobs, etc.).
func (c *Client) Raw() *generated.ClientWithResponses { return c.api }

// ProjectID returns the project the client is bound to.
func (c *Client) ProjectID() uuid.UUID { return uuid.UUID(c.projectID) }
