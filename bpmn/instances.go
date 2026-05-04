package bpmn

import (
	"context"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/QuantumBPM/quantum-go-sdk/generated"
	"github.com/QuantumBPM/quantum-go-sdk/variables"
)

// InstanceListOption tunes a list-instances query.
type InstanceListOption func(*instanceListOpts)

type instanceListOpts struct {
	pageOpts
	definitionID *openapi_types.UUID
	status       *generated.ListBpmnInstancesParamsStatus
}

// WithInstanceDefinitionID filters listings to a specific deployed definition.
func WithInstanceDefinitionID(id openapi_types.UUID) InstanceListOption {
	return func(o *instanceListOpts) { o.definitionID = &id }
}

// WithInstanceStatus filters listings to a specific lifecycle status.
func WithInstanceStatus(status string) InstanceListOption {
	s := generated.ListBpmnInstancesParamsStatus(status)
	return func(o *instanceListOpts) { o.status = &s }
}

// WithInstancePage sets the 1-indexed page number.
func WithInstancePage(p int) InstanceListOption {
	return func(o *instanceListOpts) { o.page = &p }
}

// WithInstancePageSize sets the page size.
func WithInstancePageSize(s int) InstanceListOption {
	return func(o *instanceListOpts) { o.pageSize = &s }
}

// StartInstance launches a new BPMN process instance from a deployed
// definition, returning the workflow ID assigned to it.
func (c *Client) StartInstance(ctx context.Context, processDefinitionID openapi_types.UUID, vars variables.Vars) (string, error) {
	resp, err := c.api.StartBpmnInstanceWithResponse(ctx, c.projectID, generated.StartBpmnInstanceJSONRequestBody{
		ProcessDefinitionID: processDefinitionID,
		Variables:           vars.ToWireMap(),
	})
	if err != nil {
		return "", err
	}
	if resp.JSON201 != nil && resp.JSON201.WorkflowID != nil {
		return *resp.JSON201.WorkflowID, nil
	}
	return "", errorFromResponses("start-instance", resp.StatusCode())
}

// GetInstance returns the full runtime state of an instance.
func (c *Client) GetInstance(ctx context.Context, workflowID string) (*InstanceState, error) {
	resp, err := c.api.GetBpmnInstanceWithResponse(ctx, c.projectID, workflowID)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, errorFromResponses("get-instance", resp.StatusCode())
}

// CancelInstance terminates a running instance.
func (c *Client) CancelInstance(ctx context.Context, workflowID string) error {
	resp, err := c.api.CancelBpmnInstanceWithResponse(ctx, c.projectID, workflowID)
	if err != nil {
		return err
	}
	if resp.StatusCode() >= 200 && resp.StatusCode() < 300 {
		return nil
	}
	return errorFromResponses("cancel-instance", resp.StatusCode())
}

// ListInstances returns a page of process instances in the project.
func (c *Client) ListInstances(ctx context.Context, opts ...InstanceListOption) (*InstanceListPage, error) {
	o := &instanceListOpts{}
	for _, opt := range opts {
		opt(o)
	}
	resp, err := c.api.ListBpmnInstancesWithResponse(ctx, c.projectID, &generated.ListBpmnInstancesParams{
		DefinitionID: o.definitionID,
		Status:       o.status,
		Page:         o.page,
		PageSize:     o.pageSize,
	})
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, errorFromResponses("list-instances", resp.StatusCode())
}

// GetInstanceChildren returns child instances spawned via CallActivity.
func (c *Client) GetInstanceChildren(ctx context.Context, workflowID string) (*InstanceChildren, error) {
	resp, err := c.api.GetBpmnInstanceChildrenWithResponse(ctx, c.projectID, workflowID)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, errorFromResponses("get-instance-children", resp.StatusCode())
}

// GetInstanceVariables returns the current variables of a running instance.
func (c *Client) GetInstanceVariables(ctx context.Context, workflowID string) (variables.Vars, error) {
	resp, err := c.api.GetBpmnInstanceVariablesWithResponse(ctx, c.projectID, workflowID)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return variables.FromWireMap(resp.JSON200.Variables), nil
	}
	return nil, errorFromResponses("get-instance-variables", resp.StatusCode())
}

// UpdateInstanceVariables merges the supplied variables into the instance scope.
func (c *Client) UpdateInstanceVariables(ctx context.Context, workflowID string, vars variables.Vars) error {
	resp, err := c.api.UpdateBpmnInstanceVariablesWithResponse(ctx, c.projectID, workflowID, generated.UpdateBpmnInstanceVariablesJSONRequestBody{
		Variables: map[string]interface{}(vars),
	})
	if err != nil {
		return err
	}
	if resp.StatusCode() >= 200 && resp.StatusCode() < 300 {
		return nil
	}
	return errorFromResponses("update-instance-variables", resp.StatusCode())
}

// ResolveIncident clears the named incident on the instance, optionally
// merging in supplementary variables.
func (c *Client) ResolveIncident(ctx context.Context, workflowID, incidentID string, vars variables.Vars) error {
	resp, err := c.api.ResolveBpmnIncidentWithResponse(ctx, c.projectID, workflowID, incidentID, generated.ResolveBpmnIncidentJSONRequestBody{
		Variables: vars.ToWireMap(),
	})
	if err != nil {
		return err
	}
	if resp.StatusCode() >= 200 && resp.StatusCode() < 300 {
		return nil
	}
	return errorFromResponses("resolve-incident", resp.StatusCode())
}
