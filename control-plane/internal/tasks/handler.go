package tasks

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	tasksv1 "github.com/harpia/control-plane/gen/harpia/tasks/v1"
	"github.com/harpia/control-plane/internal/workflow"
)

type TaskHandler struct {
	repo     *Repository
	temporal *workflow.TemporalClient
}

func NewTaskHandler(repo *Repository, temporal *workflow.TemporalClient) *TaskHandler {
	return &TaskHandler{repo: repo, temporal: temporal}
}

func (h *TaskHandler) CreateTask(ctx context.Context, req *connect.Request[tasksv1.CreateTaskRequest]) (*connect.Response[tasksv1.CreateTaskResponse], error) {
	tenantID, err := uuid.Parse(req.Msg.TenantId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	task := &Task{
		Title:       req.Msg.Title,
		Description: req.Msg.Description,
		Status:      "pending",
		Priority:    0,
		TenantID:    tenantID,
		CreatedBy:   uuid.Nil,
	}

	created, err := h.repo.Create(ctx, task)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if h.temporal != nil {
		input := workflow.TaskInput{
			TaskID:      created.ID.String(),
			TenantID:    created.TenantID.String(),
			Description: created.Description,
		}
		_, err := h.temporal.StartTaskWorkflow(ctx, input)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&tasksv1.CreateTaskResponse{
		Task: domainToProto(created),
	}), nil
}

func (h *TaskHandler) GetTask(ctx context.Context, req *connect.Request[tasksv1.GetTaskRequest]) (*connect.Response[tasksv1.GetTaskResponse], error) {
	tenantID, err := uuid.Parse(req.Msg.TenantId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	taskID, err := uuid.Parse(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	task, err := h.repo.GetByID(ctx, tenantID, taskID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	return connect.NewResponse(&tasksv1.GetTaskResponse{
		Task: domainToProto(task),
	}), nil
}

func (h *TaskHandler) ListTasks(ctx context.Context, req *connect.Request[tasksv1.ListTasksRequest], stream *connect.ServerStream[tasksv1.ListTasksResponse]) error {
	return nil
}

func (h *TaskHandler) WatchTask(ctx context.Context, req *connect.Request[tasksv1.WatchTaskRequest], stream *connect.ServerStream[tasksv1.WatchTaskResponse]) error {
	return nil
}

func domainToProto(t *Task) *tasksv1.Task {
	createdAt := t.CreatedAt.Format(time.RFC3339)
	updatedAt := t.UpdatedAt.Format(time.RFC3339)

	return &tasksv1.Task{
		Id:          t.ID.String(),
		TenantId:    t.TenantID.String(),
		Title:       t.Title,
		Description: t.Description,
		Status:      stringToTaskStatus(t.Status),
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}
}

func stringToTaskStatus(s string) tasksv1.TaskStatus {
	switch s {
	case "pending":
		return tasksv1.TaskStatus_TASK_STATUS_PENDING
	case "planning":
		return tasksv1.TaskStatus_TASK_STATUS_PLANNING
	case "in_progress":
		return tasksv1.TaskStatus_TASK_STATUS_IN_PROGRESS
	case "awaiting_feedback":
		return tasksv1.TaskStatus_TASK_STATUS_AWAITING_FEEDBACK
	case "completed":
		return tasksv1.TaskStatus_TASK_STATUS_COMPLETED
	case "failed":
		return tasksv1.TaskStatus_TASK_STATUS_FAILED
	case "cancelled":
		return tasksv1.TaskStatus_TASK_STATUS_CANCELLED
	default:
		return tasksv1.TaskStatus_TASK_STATUS_UNSPECIFIED
	}
}
