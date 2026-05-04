// Package bpmn wraps the QuantumBPM BPMN engine endpoints. The Client is
// project-scoped and exposes resources, instances, messaging, user tasks,
// and process discovery as method groups on a single struct.
package bpmn

import (
	"fmt"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/QuantumBPM/quantum-go-sdk/generated"
)

// Re-exported response and entity types so callers don't need to depend on
// the generated package directly.
type (
	Resource                = generated.BpmnResource
	ResourceDetail          = generated.BpmnResourceDetail
	ResourceSummary         = generated.BpmnResourceSummary
	ResourceListPage        = generated.BpmnResourcePaginatedResponse
	ResourceSummaryListPage = generated.BpmnResourceSummaryPaginatedResponse
	Instance                = generated.BpmnInstance
	InstanceState           = generated.BpmnInstanceState
	InstanceListPage        = generated.BpmnInstancePaginatedResponse
	InstanceChildren        = generated.BpmnInstanceChildrenResponse
	Incident                = generated.BpmnIncident
	ProcessSummary          = generated.BpmnProcessSummary
	ProcessSummaryPage      = generated.BpmnProcessSummaryPaginatedResponse
	ProcessVersion          = generated.BpmnProcessVersion
	ProcessVersionPage      = generated.BpmnProcessVersionPaginatedResponse
	UserTask                = generated.UserTask
	UserTaskListPage        = generated.BpmnUserTaskPaginatedResponse
	ValidationIssue         = generated.BpmnValidationIssue
	ValidateResponse        = generated.BpmnValidateResponse
	CorrelationKeys         = generated.CorrelationKeys
	UpdateAssignmentBody    = generated.UpdateUserTaskAssignmentRequest
)

// Client wraps the BPMN engine endpoints for a single project.
type Client struct {
	api       *generated.ClientWithResponses
	projectID openapi_types.UUID
}

// New constructs a BPMN client bound to projectID.
func New(api *generated.ClientWithResponses, projectID openapi_types.UUID) *Client {
	return &Client{api: api, projectID: projectID}
}

// Raw exposes the generated client for endpoints not covered by the wrapper
// (migrate, modify, ad-hoc, batch-complete, etc.). Project ID for those
// endpoints is reachable via Client.ProjectID.
func (c *Client) Raw() *generated.ClientWithResponses { return c.api }

// ProjectID returns the project the client is bound to.
func (c *Client) ProjectID() openapi_types.UUID { return c.projectID }

func errorFromResponses(op string, status int, errs ...*generated.Error) error {
	for _, e := range errs {
		if e == nil {
			continue
		}
		return fmt.Errorf("bpmn: %s: %s (%s)", op, e.Message, e.Code)
	}
	return fmt.Errorf("bpmn: %s: unexpected status %d", op, status)
}
