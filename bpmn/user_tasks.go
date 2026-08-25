package bpmn

import (
	"context"

	"github.com/QuantumBPM/quantum-go-sdk/generated"
	"github.com/QuantumBPM/quantum-go-sdk/variables"
)

// UserTaskListOption tunes a list-user-tasks query.
type UserTaskListOption func(*userTaskListOpts)

type userTaskListOpts struct {
	pageOpts
	workflowID     *string
	status         *generated.ListBpmnUserTasksParamsStatus
	assignee       *string
	candidateUser  *string
	candidateGroup *string
	businessID     *string
}

// WithUserTaskWorkflowID filters tasks by originating instance.
func WithUserTaskWorkflowID(id string) UserTaskListOption {
	return func(o *userTaskListOpts) { o.workflowID = &id }
}

// WithUserTaskStatus filters tasks by lifecycle status.
func WithUserTaskStatus(status string) UserTaskListOption {
	s := generated.ListBpmnUserTasksParamsStatus(status)
	return func(o *userTaskListOpts) { o.status = &s }
}

// WithUserTaskAssignee filters tasks by current assignee.
func WithUserTaskAssignee(user string) UserTaskListOption {
	return func(o *userTaskListOpts) { o.assignee = &user }
}

// WithUserTaskCandidateUser filters tasks where user is in candidateUsers.
func WithUserTaskCandidateUser(user string) UserTaskListOption {
	return func(o *userTaskListOpts) { o.candidateUser = &user }
}

// WithUserTaskCandidateGroup filters tasks where group is in candidateGroups.
func WithUserTaskCandidateGroup(group string) UserTaskListOption {
	return func(o *userTaskListOpts) { o.candidateGroup = &group }
}

// WithUserTaskBusinessID filters tasks by the originating instance's
// caller-supplied correlation key.
func WithUserTaskBusinessID(id string) UserTaskListOption {
	return func(o *userTaskListOpts) { o.businessID = &id }
}

// WithUserTaskPage sets the 1-indexed page number.
func WithUserTaskPage(p int) UserTaskListOption {
	return func(o *userTaskListOpts) { o.page = &p }
}

// WithUserTaskPageSize sets the page size.
func WithUserTaskPageSize(s int) UserTaskListOption {
	return func(o *userTaskListOpts) { o.pageSize = &s }
}

// ListUserTasks returns a page of user tasks in the project.
func (c *Client) ListUserTasks(ctx context.Context, opts ...UserTaskListOption) (*UserTaskListPage, error) {
	o := &userTaskListOpts{}
	for _, opt := range opts {
		opt(o)
	}
	resp, err := c.api.ListBpmnUserTasksWithResponse(ctx, c.projectID, &generated.ListBpmnUserTasksParams{
		WorkflowID:     o.workflowID,
		Status:         o.status,
		Assignee:       o.assignee,
		CandidateUser:  o.candidateUser,
		CandidateGroup: o.candidateGroup,
		BusinessId:     o.businessID,
		Page:           o.page,
		PageSize:       o.pageSize,
	})
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, errorFromResponses("list-user-tasks", resp.StatusCode())
}

// ListUserTasksForCaller returns the caller's user tasks (those they may
// claim or have already claimed).
func (c *Client) ListUserTasksForCaller(ctx context.Context, opts ...PageOption) (*UserTaskListPage, error) {
	po := applyPageOpts(opts...)
	resp, err := c.api.ListBpmnUserTasksForCallerWithResponse(ctx, c.projectID, &generated.ListBpmnUserTasksForCallerParams{
		Page: po.page, PageSize: po.pageSize,
	})
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, errorFromResponses("list-user-tasks-for-caller", resp.StatusCode())
}

// GetUserTask returns a single user task by execution key.
func (c *Client) GetUserTask(ctx context.Context, executionKey string) (*UserTask, error) {
	resp, err := c.api.GetBpmnUserTaskWithResponse(ctx, c.projectID, executionKey)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, errorFromResponses("get-user-task", resp.StatusCode())
}

// UpdateUserTaskAssignment reassigns or modifies the candidate set for a
// CREATED user task. Pass the full body - fields are replaced atomically.
func (c *Client) UpdateUserTaskAssignment(ctx context.Context, executionKey string, body UpdateAssignmentBody) (*UserTask, error) {
	resp, err := c.api.UpdateBpmnUserTaskAssignmentWithResponse(ctx, c.projectID, executionKey, body)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, errorFromResponses("update-user-task-assignment", resp.StatusCode())
}

// CompleteUserTask finalizes a user task with the supplied variables.
func (c *Client) CompleteUserTask(ctx context.Context, executionKey string, vars variables.Vars) error {
	resp, err := c.api.CompleteBpmnUserTaskWithResponse(ctx, c.projectID, executionKey, generated.CompleteBpmnUserTaskJSONRequestBody{
		Variables: vars.ToWireMap(),
	})
	if err != nil {
		return err
	}
	if resp.StatusCode() >= 200 && resp.StatusCode() < 300 {
		return nil
	}
	return errorFromResponses("complete-user-task", resp.StatusCode())
}

// ThrowUserTaskError fails a user task with a BPMN error code, optionally
// passing supplementary variables.
func (c *Client) ThrowUserTaskError(ctx context.Context, executionKey, errorCode string, vars variables.Vars) error {
	resp, err := c.api.ThrowBpmnUserTaskErrorWithResponse(ctx, c.projectID, executionKey, generated.ThrowBpmnUserTaskErrorJSONRequestBody{
		ErrorCode: errorCode,
		Variables: vars.ToWireMap(),
	})
	if err != nil {
		return err
	}
	if resp.StatusCode() >= 200 && resp.StatusCode() < 300 {
		return nil
	}
	return errorFromResponses("throw-user-task-error", resp.StatusCode())
}
