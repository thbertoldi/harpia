package plans

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/robfig/cron"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"

	"github.com/harpia/control-plane/internal/workflow"
)

const (
	scheduleIDPrefix = "plan-configuration-"
	defaultTimezone  = "UTC"
)

type ScheduleClient interface {
	Create(ctx context.Context, options client.ScheduleOptions) (client.ScheduleHandle, error)
	GetHandle(ctx context.Context, scheduleID string) client.ScheduleHandle
}

type ScheduleHandle interface {
	Describe(ctx context.Context) (*client.ScheduleDescription, error)
	Update(ctx context.Context, options client.ScheduleUpdateOptions) error
	Pause(ctx context.Context, options client.SchedulePauseOptions) error
	Delete(ctx context.Context) error
}

type ScheduleManager struct {
	schedules ScheduleClient
	logger    *slog.Logger
}

func NewScheduleManager(temporal *workflow.TemporalClient, logger *slog.Logger) *ScheduleManager {
	if logger == nil {
		logger = slog.Default()
	}

	manager := &ScheduleManager{logger: logger}
	if temporal == nil {
		return manager
	}

	manager.schedules = temporalScheduleClient{client: temporal.RawClient()}
	return manager
}

func (m *ScheduleManager) Sync(ctx context.Context, config *PlanConfiguration) error {
	if m == nil {
		return nil
	}
	if m.schedules == nil {
		m.logger.Warn("temporal schedule client unavailable, skipping schedule sync",
			"plan_configuration_id", config.ID,
		)
		return nil
	}

	scheduleID := scheduleIDForConfig(config.ID)
	handle := m.schedules.GetHandle(ctx, scheduleID)

	if shouldEnableSchedule(config) {
		cronExpr, timezone, err := scheduleFromConfig(config)
		if err != nil {
			return err
		}
		if err := validateCronExpression(cronExpr); err != nil {
			return err
		}
		return m.upsertSchedule(ctx, handle, scheduleID, config, cronExpr, timezone)
	}

	if config.Status == ConfigurationStatusDisabled {
		return m.pauseIfExists(ctx, handle, "plan configuration disabled")
	}

	return m.deleteIfExists(ctx, handle)
}

func (m *ScheduleManager) upsertSchedule(
	ctx context.Context,
	handle ScheduleHandle,
	scheduleID string,
	config *PlanConfiguration,
	cronExpr string,
	timezone string,
) error {
	action := buildScheduleAction(config)
	spec := client.ScheduleSpec{
		CronExpressions: []string{cronExpr},
		TimeZoneName:    timezone,
	}

	_, err := handle.Describe(ctx)
	if err != nil {
		if !isScheduleNotFound(err) {
			return fmt.Errorf("describe schedule %s: %w", scheduleID, err)
		}

		_, err = m.schedules.Create(ctx, client.ScheduleOptions{
			ID:     scheduleID,
			Spec:   spec,
			Action: action,
		})
		if err != nil {
			return fmt.Errorf("create schedule %s: %w", scheduleID, err)
		}
		return nil
	}

	return handle.Update(ctx, client.ScheduleUpdateOptions{
		DoUpdate: func(input client.ScheduleUpdateInput) (*client.ScheduleUpdate, error) {
			updated := input.Description.Schedule
			if updated.State == nil {
				updated.State = &client.ScheduleState{}
			}
			updated.State.Paused = false
			updated.State.Note = "plan configuration scheduled"
			updated.Spec = &spec
			updated.Action = action
			return &client.ScheduleUpdate{Schedule: &updated}, nil
		},
	})
}

func (m *ScheduleManager) pauseIfExists(ctx context.Context, handle ScheduleHandle, note string) error {
	_, err := handle.Describe(ctx)
	if err != nil {
		if isScheduleNotFound(err) {
			return nil
		}
		return fmt.Errorf("describe schedule for pause: %w", err)
	}

	if err := handle.Pause(ctx, client.SchedulePauseOptions{Note: note}); err != nil {
		return fmt.Errorf("pause schedule: %w", err)
	}
	return nil
}

func (m *ScheduleManager) deleteIfExists(ctx context.Context, handle ScheduleHandle) error {
	_, err := handle.Describe(ctx)
	if err != nil {
		if isScheduleNotFound(err) {
			return nil
		}
		return fmt.Errorf("describe schedule for delete: %w", err)
	}

	if err := handle.Delete(ctx); err != nil {
		return fmt.Errorf("delete schedule: %w", err)
	}
	return nil
}

func shouldEnableSchedule(config *PlanConfiguration) bool {
	if config.Status != ConfigurationStatusScheduled {
		return false
	}
	cronExpr, _, err := scheduleFromConfig(config)
	return err == nil && cronExpr != ""
}

func scheduleFromConfig(config *PlanConfiguration) (string, string, error) {
	if len(config.Schedule) == 0 || string(config.Schedule) == "null" {
		return "", defaultTimezone, nil
	}

	var schedule struct {
		CronExpression string `json:"cron_expression"`
		Timezone       string `json:"timezone"`
	}
	if err := json.Unmarshal(config.Schedule, &schedule); err != nil {
		return "", defaultTimezone, fmt.Errorf("parse schedule: %w", err)
	}

	timezone := schedule.Timezone
	if timezone == "" {
		timezone = defaultTimezone
	}
	return schedule.CronExpression, timezone, nil
}

func validateScheduleForStatus(status string, scheduleJSON json.RawMessage) error {
	if status != ConfigurationStatusScheduled {
		return nil
	}

	cronExpr, _, err := scheduleFromRaw(scheduleJSON)
	if err != nil {
		return err
	}
	if cronExpr == "" {
		return errors.New("cron_expression is required when status is scheduled")
	}
	return validateCronExpression(cronExpr)
}

func scheduleFromRaw(scheduleJSON json.RawMessage) (string, string, error) {
	if len(scheduleJSON) == 0 || string(scheduleJSON) == "null" {
		return "", defaultTimezone, nil
	}

	var schedule struct {
		CronExpression string `json:"cron_expression"`
		Timezone       string `json:"timezone"`
	}
	if err := json.Unmarshal(scheduleJSON, &schedule); err != nil {
		return "", defaultTimezone, fmt.Errorf("parse schedule: %w", err)
	}

	timezone := schedule.Timezone
	if timezone == "" {
		timezone = defaultTimezone
	}
	return schedule.CronExpression, timezone, nil
}

func validateCronExpression(expression string) error {
	if expression == "" {
		return errors.New("cron_expression is required")
	}
	if _, err := cron.ParseStandard(expression); err != nil {
		return fmt.Errorf("invalid cron_expression: %w", err)
	}
	return nil
}

func scheduleIDForConfig(configID uuid.UUID) string {
	return scheduleIDPrefix + configID.String()
}

func buildScheduleAction(config *PlanConfiguration) *client.ScheduleWorkflowAction {
	return &client.ScheduleWorkflowAction{
		ID:        fmt.Sprintf("plan-execution-%s", config.ID),
		Workflow:  workflow.PlanScheduledExecutionWorkflowName,
		TaskQueue: workflow.TaskQueueName,
		Args: []interface{}{
			workflow.PlanExecutionInput{
				TenantID:            config.TenantID.String(),
				PlanConfigurationID: config.ID.String(),
			},
		},
	}
}

func isScheduleNotFound(err error) bool {
	var notFound *serviceerror.NotFound
	return errors.As(err, &notFound)
}

type temporalScheduleClient struct {
	client client.Client
}

func (c temporalScheduleClient) Create(ctx context.Context, options client.ScheduleOptions) (client.ScheduleHandle, error) {
	return c.client.ScheduleClient().Create(ctx, options)
}

func (c temporalScheduleClient) GetHandle(ctx context.Context, scheduleID string) client.ScheduleHandle {
	return c.client.ScheduleClient().GetHandle(ctx, scheduleID)
}
