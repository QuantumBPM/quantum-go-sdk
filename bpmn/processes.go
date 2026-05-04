package bpmn

import (
	"context"
	"time"

	"github.com/QuantumBPM/quantum-go-sdk/generated"
)

// ProcessListOption tunes a list-processes query.
type ProcessListOption func(*processListOpts)

type processListOpts struct {
	pageOpts
	q            *string
	createdAfter *time.Time
}

// WithProcessSearch filters by case-insensitive substring match on process id
// or name.
func WithProcessSearch(q string) ProcessListOption {
	return func(o *processListOpts) { o.q = &q }
}

// WithProcessCreatedAfter bounds the totalCount aggregate to instances
// created at or after the supplied timestamp.
func WithProcessCreatedAfter(t time.Time) ProcessListOption {
	return func(o *processListOpts) { o.createdAfter = &t }
}

// WithProcessPage sets the 1-indexed page number.
func WithProcessPage(p int) ProcessListOption {
	return func(o *processListOpts) { o.page = &p }
}

// WithProcessPageSize sets the page size.
func WithProcessPageSize(s int) ProcessListOption {
	return func(o *processListOpts) { o.pageSize = &s }
}

// ListProcesses returns a page of process summaries (one per process id).
func (c *Client) ListProcesses(ctx context.Context, opts ...ProcessListOption) (*ProcessSummaryPage, error) {
	o := &processListOpts{}
	for _, opt := range opts {
		opt(o)
	}
	resp, err := c.api.ListBpmnProcessesWithResponse(ctx, c.projectID, &generated.ListBpmnProcessesParams{
		Page:         o.page,
		PageSize:     o.pageSize,
		Q:            o.q,
		CreatedAfter: o.createdAfter,
	})
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, errorFromResponses("list-processes", resp.StatusCode())
}

// ListProcessVersions returns every deployed version of a process id.
func (c *Client) ListProcessVersions(ctx context.Context, processID string, opts ...ProcessListOption) (*ProcessVersionPage, error) {
	o := &processListOpts{}
	for _, opt := range opts {
		opt(o)
	}
	resp, err := c.api.ListBpmnProcessVersionsWithResponse(ctx, c.projectID, processID, &generated.ListBpmnProcessVersionsParams{
		Page:         o.page,
		PageSize:     o.pageSize,
		CreatedAfter: o.createdAfter,
	})
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, errorFromResponses("list-process-versions", resp.StatusCode())
}
