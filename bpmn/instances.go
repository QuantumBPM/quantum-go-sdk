package bpmn

import (
	"context"

	"github.com/QuantumBPM/quantum-go-sdk/v2/generated"
	"github.com/QuantumBPM/quantum-go-sdk/v2/variables"
)

// InstanceListOption tunes a list-instances query.
type InstanceListOption func(*instanceListOpts)

type instanceListOpts struct {
	cursorOpts
	processID  *string
	version    *int
	status     *generated.ListBpmnInstancesParamsStatus
	businessID *string
}

// WithInstanceProcess filters listings to one process, in every version.
func WithInstanceProcess(processID string) InstanceListOption {
	return func(o *instanceListOpts) { o.processID = &processID }
}

// WithInstanceVersion narrows a WithInstanceProcess listing to one version.
func WithInstanceVersion(version int) InstanceListOption {
	return func(o *instanceListOpts) { o.version = &version }
}

// WithInstanceStatus filters listings to a specific lifecycle status.
func WithInstanceStatus(status string) InstanceListOption {
	s := generated.ListBpmnInstancesParamsStatus(status)
	return func(o *instanceListOpts) { o.status = &s }
}

// WithInstanceBusinessID filters listings to a single caller-supplied
// correlation key (the value passed to StartInstance via WithStartBusinessID).
func WithInstanceBusinessID(id string) InstanceListOption {
	return func(o *instanceListOpts) { o.businessID = &id }
}

// WithInstanceCursor continues a listing from the page before's
// Pagination.NextCursor.
func WithInstanceCursor(cursor string) InstanceListOption {
	return func(o *instanceListOpts) { o.cursor = &cursor }
}

// WithInstanceLimit sets the most instances a page returns, 20 by default
// and 100 at most.
func WithInstanceLimit(limit int) InstanceListOption {
	return func(o *instanceListOpts) { o.limit = &limit }
}

// StartInstanceOption tunes a StartInstance / StartTestInstance call.
type StartInstanceOption func(*startInstanceOpts)

type startInstanceOpts struct {
	businessID *string
	version    *int
}

// WithStartBusinessID stamps the new instance with a caller-supplied
// correlation key (order number, ticket ID, etc.). Inherited by every child
// instance, external job, user task, and DMN execution emitted by it.
func WithStartBusinessID(id string) StartInstanceOption {
	return func(o *startInstanceOpts) { o.businessID = &id }
}

// WithStartVersion starts the given deployed version instead of the latest.
// StartTestInstance ignores it.
func WithStartVersion(version int) StartInstanceOption {
	return func(o *startInstanceOpts) { o.version = &version }
}

func applyStartInstanceOpts(opts ...StartInstanceOption) startInstanceOpts {
	o := startInstanceOpts{}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// StartInstance launches a new instance of a deployed BPMN process, its
// latest version unless WithStartVersion names one, and returns the ID
// assigned to it.
func (c *Client) StartInstance(ctx context.Context, processID string, vars variables.Vars, opts ...StartInstanceOption) (string, error) {
	o := applyStartInstanceOpts(opts...)
	resp, err := c.api.StartBpmnInstanceWithResponse(ctx, c.projectID, generated.StartBpmnInstanceJSONRequestBody{
		ProcessID:  processID,
		Version:    o.version,
		Variables:  vars.ToWireMap(),
		BusinessId: o.businessID,
	})
	if err != nil {
		return "", err
	}
	if resp.JSON201 != nil && resp.JSON201.InstanceID != nil {
		return *resp.JSON201.InstanceID, nil
	}
	return "", errorFromResponses("start-instance", resp.StatusCode())
}

// GetInstance returns the full runtime state of an instance.
func (c *Client) GetInstance(ctx context.Context, instanceID string) (*InstanceState, error) {
	resp, err := c.api.GetBpmnInstanceWithResponse(ctx, c.projectID, instanceID)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, errorFromResponses("get-instance", resp.StatusCode())
}

// CancelInstance terminates a running instance.
func (c *Client) CancelInstance(ctx context.Context, instanceID string) error {
	resp, err := c.api.CancelBpmnInstanceWithResponse(ctx, c.projectID, instanceID)
	if err != nil {
		return err
	}
	if resp.StatusCode() >= 200 && resp.StatusCode() < 300 {
		return nil
	}
	return errorFromResponses("cancel-instance", resp.StatusCode())
}

// ListInstances returns a page of process instances in the project, newest
// first. Pass the page's Pagination.NextCursor to WithInstanceCursor for the
// next one. It is nil on the last page.
func (c *Client) ListInstances(ctx context.Context, opts ...InstanceListOption) (*InstanceListPage, error) {
	o := &instanceListOpts{}
	for _, opt := range opts {
		opt(o)
	}
	resp, err := c.api.ListBpmnInstancesWithResponse(ctx, c.projectID, &generated.ListBpmnInstancesParams{
		ProcessId:  o.processID,
		Version:    o.version,
		Status:     o.status,
		BusinessId: o.businessID,
		Cursor:     o.cursor,
		Limit:      o.limit,
	})
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, errorFromResponses("list-instances", resp.StatusCode())
}

// ChildrenOption tunes a GetInstanceChildren read.
type ChildrenOption func(*cursorOpts)

// WithChildrenCursor continues a listing from the page before's
// Pagination.NextCursor.
func WithChildrenCursor(cursor string) ChildrenOption {
	return func(o *cursorOpts) { o.cursor = &cursor }
}

// WithChildrenLimit sets the most instances a page returns, 20 by default
// and 100 at most.
func WithChildrenLimit(limit int) ChildrenOption {
	return func(o *cursorOpts) { o.limit = &limit }
}

// GetInstanceChildren returns a page of the instances this instance spawned
// via CallActivity, newest first. Pass the page's Pagination.NextCursor to
// WithChildrenCursor for the next one. It is nil on the last page.
func (c *Client) GetInstanceChildren(ctx context.Context, instanceID string, opts ...ChildrenOption) (*InstanceListPage, error) {
	o := &cursorOpts{}
	for _, opt := range opts {
		opt(o)
	}
	resp, err := c.api.GetBpmnInstanceChildrenWithResponse(ctx, c.projectID, instanceID, &generated.GetBpmnInstanceChildrenParams{
		Cursor: o.cursor,
		Limit:  o.limit,
	})
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, errorFromResponses("get-instance-children", resp.StatusCode())
}

// HistoryOption tunes a GetInstanceHistory read.
type HistoryOption func(*historyOpts)

type historyOpts struct {
	fromSeq   *int64
	beforeSeq *int64
	limit     *int
}

// WithHistoryFromSeq reads the entries from seq on, to follow a running
// instance. It takes precedence over WithHistoryBeforeSeq.
func WithHistoryFromSeq(seq int64) HistoryOption {
	return func(o *historyOpts) { o.fromSeq = &seq }
}

// WithHistoryBeforeSeq reads the newest entries before seq, to walk back.
func WithHistoryBeforeSeq(seq int64) HistoryOption {
	return func(o *historyOpts) { o.beforeSeq = &seq }
}

// WithHistoryLimit sets the most entries a page returns.
func WithHistoryLimit(limit int) HistoryOption {
	return func(o *historyOpts) { o.limit = &limit }
}

// GetInstanceHistory returns a page of an instance's history, oldest first,
// for a running or an ended instance. Without options the page holds the
// newest entries. Page entries leave out their inputs and outputs, which
// GetInstanceHistoryEntry reads.
func (c *Client) GetInstanceHistory(ctx context.Context, instanceID string, opts ...HistoryOption) (*HistoryPage, error) {
	o := &historyOpts{}
	for _, opt := range opts {
		opt(o)
	}
	resp, err := c.api.GetBpmnInstanceHistoryWithResponse(ctx, c.projectID, instanceID, &generated.GetBpmnInstanceHistoryParams{
		FromSeq:   o.fromSeq,
		BeforeSeq: o.beforeSeq,
		Limit:     o.limit,
	})
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, errorFromResponses("get-instance-history", resp.StatusCode(), resp.JSON404)
}

// GetInstanceHistoryEntry returns the history entry seq with its inputs and
// outputs.
func (c *Client) GetInstanceHistoryEntry(ctx context.Context, instanceID string, seq int64) (*HistoryEntry, error) {
	resp, err := c.api.GetBpmnInstanceHistoryEntryWithResponse(ctx, c.projectID, instanceID, seq)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, errorFromResponses("get-instance-history-entry", resp.StatusCode(), resp.JSON404)
}

// GetInstanceVariables returns the current variables of a running instance.
func (c *Client) GetInstanceVariables(ctx context.Context, instanceID string) (variables.Vars, error) {
	resp, err := c.api.GetBpmnInstanceVariablesWithResponse(ctx, c.projectID, instanceID)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return variables.FromWireMap(resp.JSON200.Variables), nil
	}
	return nil, errorFromResponses("get-instance-variables", resp.StatusCode())
}

// UpdateInstanceVariables merges the supplied variables into the instance scope.
func (c *Client) UpdateInstanceVariables(ctx context.Context, instanceID string, vars variables.Vars) error {
	resp, err := c.api.UpdateBpmnInstanceVariablesWithResponse(ctx, c.projectID, instanceID, generated.UpdateBpmnInstanceVariablesJSONRequestBody{
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
func (c *Client) ResolveIncident(ctx context.Context, instanceID, incidentID string, vars variables.Vars) error {
	resp, err := c.api.ResolveBpmnIncidentWithResponse(ctx, c.projectID, instanceID, incidentID, generated.ResolveBpmnIncidentJSONRequestBody{
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
