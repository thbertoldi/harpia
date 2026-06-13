package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	tasksv1 "github.com/harpia/control-plane/gen/harpia/tasks/v1"
	"github.com/harpia/control-plane/internal/cache"
	"github.com/harpia/control-plane/internal/identity"
	"github.com/harpia/control-plane/internal/workflow"
)

type TaskHandler struct {
	repo      *Repository
	temporal  *workflow.TemporalClient
	cache     *cache.TenantStore
	devUserID uuid.UUID
}

func NewTaskHandler(repo *Repository, temporal *workflow.TemporalClient, cacheStore *cache.TenantStore, devUserID uuid.UUID) (*TaskHandler, error) {
	if repo == nil {
		return nil, errors.New("tasks: repository is required")
	}
	return &TaskHandler{repo: repo, temporal: temporal, cache: cacheStore, devUserID: devUserID}, nil
}

func (h *TaskHandler) CreateTask(ctx context.Context, req *connect.Request[tasksv1.CreateTaskRequest]) (*connect.Response[tasksv1.CreateTaskResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}

	task := &Task{
		Title:       req.Msg.Title,
		Description: req.Msg.Description,
		Status:      TaskStatusPending,
		Priority:    0,
		TenantID:    tenantID,
		CreatedBy:   h.devUserID,
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
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}

	taskID, err := uuid.Parse(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if h.cache != nil {
		cacheKey := fmt.Sprintf("task:%s", taskID.String())
		if cached, cacheErr := h.cache.Get(ctx, cacheKey); cacheErr == nil {
			var task Task
			if json.Unmarshal([]byte(cached), &task) == nil {
				return connect.NewResponse(&tasksv1.GetTaskResponse{
					Task: domainToProto(&task),
				}), nil
			}
		}
	}

	task, err := h.repo.GetByID(ctx, tenantID, taskID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	if h.cache != nil {
		cacheKey := fmt.Sprintf("task:%s", taskID.String())
		if data, marshalErr := json.Marshal(task); marshalErr == nil {
			_ = h.cache.Set(ctx, cacheKey, string(data), 5*time.Minute)
		}
	}

	return connect.NewResponse(&tasksv1.GetTaskResponse{
		Task: domainToProto(task),
	}), nil
}

func (h *TaskHandler) ListTasks(ctx context.Context, req *connect.Request[tasksv1.ListTasksRequest], stream *connect.ServerStream[tasksv1.ListTasksResponse]) error {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return err
	}

	status := ""
	if req.Msg.Status != nil && *req.Msg.Status != tasksv1.TaskStatus_TASK_STATUS_UNSPECIFIED {
		var statusErr error
		status, statusErr = taskStatusToString(*req.Msg.Status)
		if statusErr != nil {
			return connect.NewError(connect.CodeInvalidArgument, statusErr)
		}
	}
	limit := int(req.Msg.PageSize)
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset := 0
	if req.Msg.PageToken != "" {
		parsed, parseErr := strconv.Atoi(req.Msg.PageToken)
		if parseErr != nil || parsed < 0 {
			return connect.NewError(
				connect.CodeInvalidArgument,
				fmt.Errorf("invalid page token %q", req.Msg.PageToken),
			)
		}
		offset = parsed
	}

	tasks, err := h.repo.List(ctx, tenantID, status, limit+1, offset)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	hasNext := len(tasks) > limit
	if hasNext {
		tasks = tasks[:limit]
	}
	response := &tasksv1.ListTasksResponse{Tasks: make([]*tasksv1.Task, 0, len(tasks))}
	for i := range tasks {
		response.Tasks = append(response.Tasks, domainToProto(&tasks[i]))
	}
	if hasNext {
		response.NextPageToken = strconv.Itoa(offset + limit)
	}
	return stream.Send(response)
}

func (h *TaskHandler) WatchTask(ctx context.Context, req *connect.Request[tasksv1.WatchTaskRequest], stream *connect.ServerStream[tasksv1.WatchTaskResponse]) error {
	_, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	return err
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

func taskStatusToString(status tasksv1.TaskStatus) (string, error) {
	switch status {
	case tasksv1.TaskStatus_TASK_STATUS_PENDING:
		return TaskStatusPending, nil
	case tasksv1.TaskStatus_TASK_STATUS_PLANNING:
		return TaskStatusPlanning, nil
	case tasksv1.TaskStatus_TASK_STATUS_IN_PROGRESS:
		return TaskStatusInProgress, nil
	case tasksv1.TaskStatus_TASK_STATUS_AWAITING_FEEDBACK:
		return TaskStatusAwaitingFeedback, nil
	case tasksv1.TaskStatus_TASK_STATUS_COMPLETED:
		return TaskStatusCompleted, nil
	case tasksv1.TaskStatus_TASK_STATUS_FAILED:
		return TaskStatusFailed, nil
	case tasksv1.TaskStatus_TASK_STATUS_CANCELLED:
		return TaskStatusCancelled, nil
	default:
		return "", fmt.Errorf("unsupported task status %s", status.String())
	}
}

func stringToTaskStatus(s string) tasksv1.TaskStatus {
	switch s {
	case TaskStatusPending:
		return tasksv1.TaskStatus_TASK_STATUS_PENDING
	case TaskStatusPlanning:
		return tasksv1.TaskStatus_TASK_STATUS_PLANNING
	case TaskStatusInProgress:
		return tasksv1.TaskStatus_TASK_STATUS_IN_PROGRESS
	case TaskStatusAwaitingFeedback:
		return tasksv1.TaskStatus_TASK_STATUS_AWAITING_FEEDBACK
	case TaskStatusCompleted:
		return tasksv1.TaskStatus_TASK_STATUS_COMPLETED
	case TaskStatusFailed:
		return tasksv1.TaskStatus_TASK_STATUS_FAILED
	case TaskStatusCancelled:
		return tasksv1.TaskStatus_TASK_STATUS_CANCELLED
	default:
		return tasksv1.TaskStatus_TASK_STATUS_UNSPECIFIED
	}
}
