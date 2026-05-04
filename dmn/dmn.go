// Package dmn wraps the QuantumBPM DMN evaluation endpoints with a
// project-scoped client. Use Client.Evaluate for stored definitions and
// Client.EvaluateDesign for ad-hoc XML.
package dmn

import (
	"context"
	"fmt"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/QuantumBPM/quantum-go-sdk/generated"
	"github.com/QuantumBPM/quantum-go-sdk/variables"
)

// Result maps decision IDs to the result of evaluating that decision.
type Result = map[string]generated.EvaluationResult

// BatchResult is one result row per input row in a batch evaluation.
type BatchResult = generated.BatchEvaluationResponse

// Client evaluates DMN definitions in a single project.
type Client struct {
	api       *generated.ClientWithResponses
	projectID openapi_types.UUID
}

// New constructs a DMN client bound to projectID. api is the authenticated
// generated client (typically obtained from auth.NewClient).
func New(api *generated.ClientWithResponses, projectID openapi_types.UUID) *Client {
	return &Client{api: api, projectID: projectID}
}

// EvaluateOption tunes a stored DMN evaluation.
type EvaluateOption func(*evaluateOpts)

type evaluateOpts struct {
	version          *int
	decisions        *[]string
	decisionServices *[]string
}

// WithVersion pins the evaluation to a specific version of the definition.
// Without this option, the latest version is used.
func WithVersion(v int) EvaluateOption {
	return func(o *evaluateOpts) { o.version = &v }
}

// WithDecisions restricts evaluation to the named decisions in the document.
// Without this option, every decision is evaluated.
func WithDecisions(names ...string) EvaluateOption {
	return func(o *evaluateOpts) { o.decisions = &names }
}

// WithDecisionServices selects decision services to evaluate.
func WithDecisionServices(names ...string) EvaluateOption {
	return func(o *evaluateOpts) { o.decisionServices = &names }
}

// Evaluate runs a stored DMN definition identified by its DMN XML
// `<definitions id="…">` value. This is the typical evaluation path: stable
// across deployed versions, addressable by business identifiers from the
// model.
func (c *Client) Evaluate(ctx context.Context, definitionsID string, vars variables.Vars, opts ...EvaluateOption) (Result, error) {
	o := &evaluateOpts{}
	for _, opt := range opts {
		opt(o)
	}

	fctx, err := vars.ToFeelContext()
	if err != nil {
		return nil, fmt.Errorf("dmn: build context: %w", err)
	}

	body := generated.EvaluateByDefinitionsIDJSONRequestBody{
		Context:          fctx,
		Version:          o.version,
		Decisions:        o.decisions,
		DecisionServices: o.decisionServices,
	}
	params := &generated.EvaluateByDefinitionsIDParams{Version: o.version}

	resp, err := c.api.EvaluateByDefinitionsIDWithResponse(ctx, c.projectID, definitionsID, params, body)
	if err != nil {
		return nil, fmt.Errorf("dmn: evaluate request: %w", err)
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return nil, errorFromResponses("evaluate", resp.StatusCode(),
		resp.JSON400, resp.JSON404, resp.JSON500)
}

// EvaluateByID runs a stored DMN definition addressed by its platform UUID.
// Prefer Evaluate (by definitions ID) for normal use; this overload is for
// callers that already hold a database-version pointer.
func (c *Client) EvaluateByID(ctx context.Context, definitionID openapi_types.UUID, vars variables.Vars, opts ...EvaluateOption) (Result, error) {
	o := &evaluateOpts{}
	for _, opt := range opts {
		opt(o)
	}

	fctx, err := vars.ToFeelContext()
	if err != nil {
		return nil, fmt.Errorf("dmn: build context: %w", err)
	}

	body := generated.EvaluateStoredJSONRequestBody{
		Context:          fctx,
		Version:          o.version,
		Decisions:        o.decisions,
		DecisionServices: o.decisionServices,
	}

	resp, err := c.api.EvaluateStoredWithResponse(ctx, c.projectID, definitionID, body)
	if err != nil {
		return nil, fmt.Errorf("dmn: evaluate request: %w", err)
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return nil, errorFromResponses("evaluate", resp.StatusCode(),
		resp.JSON400, resp.JSON404, resp.JSON500)
}

// DesignOption tunes an ad-hoc design evaluation.
type DesignOption func(*designOpts)

type designOpts struct {
	decisions        *[]string
	decisionServices *[]string
	additionalXMLs   *[]string
}

// WithDesignDecisions restricts a design evaluation to the named decisions.
func WithDesignDecisions(names ...string) DesignOption {
	return func(o *designOpts) { o.decisions = &names }
}

// WithDesignDecisionServices selects decision services to evaluate.
func WithDesignDecisionServices(names ...string) DesignOption {
	return func(o *designOpts) { o.decisionServices = &names }
}

// WithAdditionalXMLs supplies extra DMN documents whose decisions can be
// imported and referenced from the primary xml.
func WithAdditionalXMLs(xmls ...string) DesignOption {
	return func(o *designOpts) { o.additionalXMLs = &xmls }
}

// EvaluateDesign runs ad-hoc DMN XML against an input context. The XML is
// not stored; useful for "evaluate while editing" flows.
func (c *Client) EvaluateDesign(ctx context.Context, xml string, vars variables.Vars, opts ...DesignOption) (Result, error) {
	o := &designOpts{}
	for _, opt := range opts {
		opt(o)
	}

	fctx, err := vars.ToFeelContext()
	if err != nil {
		return nil, fmt.Errorf("dmn: build context: %w", err)
	}

	body := generated.EvaluateDesignJSONRequestBody{
		Xml:              xml,
		Context:          &fctx,
		Decisions:        o.decisions,
		DecisionServices: o.decisionServices,
		AdditionalXMLs:   o.additionalXMLs,
	}
	resp, err := c.api.EvaluateDesignWithResponse(ctx, body)
	if err != nil {
		return nil, fmt.Errorf("dmn: evaluate-design request: %w", err)
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return nil, errorFromResponses("evaluate-design", resp.StatusCode(),
		resp.JSON400, nil, resp.JSON500)
}

// EvaluateDesignBatch evaluates the same XML against many input rows in a
// single request, returning per-row results.
func (c *Client) EvaluateDesignBatch(ctx context.Context, xml string, rows []variables.Vars) (BatchResult, error) {
	inputs := make([]map[string]interface{}, len(rows))
	for i, row := range rows {
		inputs[i] = map[string]interface{}(row)
	}
	body := generated.EvaluateDesignBatchJSONRequestBody{
		Xml:    &xml,
		Inputs: &inputs,
	}
	resp, err := c.api.EvaluateDesignBatchWithResponse(ctx, body)
	if err != nil {
		return BatchResult{}, fmt.Errorf("dmn: batch-evaluate-design request: %w", err)
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return BatchResult{}, errorFromResponses("evaluate-design-batch", resp.StatusCode(),
		resp.JSON400, nil, resp.JSON500)
}

func errorFromResponses(op string, status int, e400, e404, e500 *generated.Error) error {
	switch {
	case e400 != nil:
		return fmt.Errorf("dmn: %s: bad request: %s", op, e400.Message)
	case e404 != nil:
		return fmt.Errorf("dmn: %s: not found: %s", op, e404.Message)
	case e500 != nil:
		return fmt.Errorf("dmn: %s: server error: %s", op, e500.Message)
	}
	return fmt.Errorf("dmn: %s: unexpected status %d", op, status)
}
