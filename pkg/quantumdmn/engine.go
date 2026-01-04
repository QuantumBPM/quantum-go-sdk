package quantumdmn

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// EngineClient provides a high-level API for interacting with the DMN engine.
// It wraps the generated ClientWithResponses and binds it to a specific Project ID.
type EngineClient struct {
	Client    *ClientWithResponses
	projectID openapi_types.UUID
}

// NewEngineClient creates a new EngineClient.
func NewEngineClient(baseURL string, projectID uuid.UUID, tokenProvider TokenProvider) (*EngineClient, error) {
	client, err := NewAuthenticatedClient(baseURL, tokenProvider)
	if err != nil {
		return nil, err
	}

	return &EngineClient{
		Client:    client,
		projectID: openapi_types.UUID(projectID),
	}, nil
}

// Evaluate performs a DMN evaluation using the XML Definition ID.
// This is the primary method for interacting with the engine.
func (c *EngineClient) Evaluate(ctx context.Context, xmlId string, version *int, evalContext map[string]interface{}) (map[string]EvaluationResult, error) {
	fCtx, err := toFeelContext(evalContext)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize context: %w", err)
	}

	params := &EvaluateByXMLIDParams{
		Version: version,
	}

	body := EvaluateByXMLIDJSONRequestBody{
		Context: fCtx,
	}

	resp, err := c.Client.EvaluateByXMLIDWithResponse(ctx, c.projectID, xmlId, params, body)
	if err != nil {
		return nil, fmt.Errorf("evaluation request failed: %w", err)
	}

	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}

	if resp.JSON400 != nil {
		msg := "bad request"
		if resp.JSON400.Message != nil {
			msg = fmt.Sprintf("bad request: %s", *resp.JSON400.Message)
		}
		return nil, fmt.Errorf(msg)
	}
	if resp.JSON404 != nil {
		msg := "definition not found"
		if resp.JSON404.Message != nil {
			msg = fmt.Sprintf("definition not found: %s", *resp.JSON404.Message)
		}
		return nil, fmt.Errorf(msg)
	}
	if resp.JSON500 != nil {
		msg := "internal server error"
		if resp.JSON500.Message != nil {
			msg = fmt.Sprintf("internal server error: %s", *resp.JSON500.Message)
		}
		return nil, fmt.Errorf(msg)
	}

	return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode())
}

func toFeelContext(ctx map[string]interface{}) (FeelContext, error) {
	b, err := json.Marshal(ctx)
	if err != nil {
		return nil, err
	}
	var fCtx FeelContext
	if err := json.Unmarshal(b, &fCtx); err != nil {
		return nil, err
	}
	return fCtx, nil
}
