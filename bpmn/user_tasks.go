package bpmn

import (
	"context"
	"time"

	"github.com/QuantumBPM/quantum-go-sdk/v2/generated"
	"github.com/QuantumBPM/quantum-go-sdk/v2/variables"
)

// UserTaskListOption tunes a list-user-tasks query.
type UserTaskListOption func(*userTaskListOpts)

type userTaskListOpts struct {
	cursorOpts
	instanceID     *string
	status         *generated.ListBpmnUserTasksParamsStatus
	assignee       *string
	candidateUser  *string
	candidateGroup *string
	businessID     *string
	dueAfter       *time.Time
	dueBefore      *time.Time
	sort           *string
}

// WithUserTaskInstanceID filters tasks by originating instance.
func WithUserTaskInstanceID(id string) UserTaskListOption {
	return func(o *userTaskListOpts) { o.instanceID = &id }
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

// WithUserTaskDueAfter keeps the tasks due at or after t.
func WithUserTaskDueAfter(t time.Time) UserTaskListOption {
	return func(o *userTaskListOpts) { o.dueAfter = &t }
}

// WithUserTaskDueBefore keeps the tasks due before t. With status CREATED
// and the current time, the overdue tasks.
func WithUserTaskDueBefore(t time.Time) UserTaskListOption {
	return func(o *userTaskListOpts) { o.dueBefore = &t }
}

// WithUserTaskSort orders the list: "created" newest first (the default),
// "dueDate" soonest due first, "priority" highest first. A cursor continues
// only a list of the same sort.
func WithUserTaskSort(sort string) UserTaskListOption {
	return func(o *userTaskListOpts) { o.sort = &sort }
}

// WithUserTaskCursor continues a listing from the page before's
// Pagination.NextCursor.
func WithUserTaskCursor(cursor string) UserTaskListOption {
	return func(o *userTaskListOpts) { o.cursor = &cursor }
}

// WithUserTaskLimit sets the most tasks a page returns, 20 by default and
// 100 at most.
func WithUserTaskLimit(limit int) UserTaskListOption {
	return func(o *userTaskListOpts) { o.limit = &limit }
}

// ListUserTasks returns a page of user tasks in the project. Pass the page's
// Pagination.NextCursor to WithUserTaskCursor for the next one. It is nil on
// the last page.
func (c *Client) ListUserTasks(ctx context.Context, opts ...UserTaskListOption) (*UserTaskListPage, error) {
	o := &userTaskListOpts{}
	for _, opt := range opts {
		opt(o)
	}
	resp, err := c.api.ListBpmnUserTasksWithResponse(ctx, c.projectID, &generated.ListBpmnUserTasksParams{
		InstanceID:     o.instanceID,
		Status:         o.status,
		Assignee:       o.assignee,
		CandidateUser:  o.candidateUser,
		CandidateGroup: o.candidateGroup,
		BusinessId:     o.businessID,
		DueAfter:       o.dueAfter,
		DueBefore:      o.dueBefore,
		Sort:           (*generated.ListBpmnUserTasksParamsSort)(o.sort),
		Cursor:         o.cursor,
		Limit:          o.limit,
	})
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, errorFromResponses("list-user-tasks", resp.StatusCode())
}

// ListUserTasksForCaller returns a page of the caller's user tasks (those
// they may claim or have already claimed). It takes the due date, sort,
// cursor and limit options. The other filters do not apply to it.
func (c *Client) ListUserTasksForCaller(ctx context.Context, opts ...UserTaskListOption) (*UserTaskListPage, error) {
	o := &userTaskListOpts{}
	for _, opt := range opts {
		opt(o)
	}
	resp, err := c.api.ListBpmnUserTasksForCallerWithResponse(ctx, c.projectID, &generated.ListBpmnUserTasksForCallerParams{
		DueAfter:  o.dueAfter,
		DueBefore: o.dueBefore,
		Sort:      (*generated.ListBpmnUserTasksForCallerParamsSort)(o.sort),
		Cursor:    o.cursor,
		Limit:     o.limit,
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
