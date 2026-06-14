package plans

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"

	"github.com/harpia/control-plane/internal/workflow"
)

type mockScheduleClient struct {
	createCalls int
	handles     map[string]*mockScheduleHandle
}

func (m *mockScheduleClient) Create(ctx context.Context, options client.ScheduleOptions) (client.ScheduleHandle, error) {
	m.createCalls++
	handle := &mockScheduleHandle{id: options.ID}
	if m.handles == nil {
		m.handles = make(map[string]*mockScheduleHandle)
	}
	m.handles[options.ID] = handle
	return handle, nil
}

func (m *mockScheduleClient) GetHandle(ctx context.Context, scheduleID string) client.ScheduleHandle {
	if m.handles == nil {
		m.handles = make(map[string]*mockScheduleHandle)
	}
	handle, ok := m.handles[scheduleID]
	if !ok {
		handle = &mockScheduleHandle{id: scheduleID, notFound: true}
		m.handles[scheduleID] = handle
	}
	return handle
}

type mockScheduleHandle struct {
	id       string
	notFound bool
	paused   bool
	deleted  bool
	updated  bool
}

func (h *mockScheduleHandle) GetID() string {
	return h.id
}

func (h *mockScheduleHandle) Describe(ctx context.Context) (*client.ScheduleDescription, error) {
	if h.notFound || h.deleted {
		return nil, &serviceerror.NotFound{}
	}
	return &client.ScheduleDescription{
		Schedule: client.Schedule{
			State: &client.ScheduleState{Paused: h.paused},
		},
	}, nil
}

func (h *mockScheduleHandle) Update(ctx context.Context, options client.ScheduleUpdateOptions) error {
	if h.notFound || h.deleted {
		return &serviceerror.NotFound{}
	}
	if options.DoUpdate == nil {
		return errors.New("missing DoUpdate")
	}
	_, err := options.DoUpdate(client.ScheduleUpdateInput{
		Description: client.ScheduleDescription{
			Schedule: client.Schedule{
				State: &client.ScheduleState{},
			},
		},
	})
	if err != nil {
		return err
	}
	h.updated = true
	h.paused = false
	return nil
}

func (h *mockScheduleHandle) Pause(ctx context.Context, options client.SchedulePauseOptions) error {
	if h.notFound || h.deleted {
		return &serviceerror.NotFound{}
	}
	h.paused = true
	return nil
}

func (h *mockScheduleHandle) Delete(ctx context.Context) error {
	if h.notFound {
		return &serviceerror.NotFound{}
	}
	h.deleted = true
	return nil
}

func (h *mockScheduleHandle) Backfill(ctx context.Context, options client.ScheduleBackfillOptions) error {
	return errors.New("not implemented")
}

func (h *mockScheduleHandle) Trigger(ctx context.Context, options client.ScheduleTriggerOptions) error {
	return errors.New("not implemented")
}

func (h *mockScheduleHandle) Unpause(ctx context.Context, options client.ScheduleUnpauseOptions) error {
	return errors.New("not implemented")
}

func scheduledConfig() *PlanConfiguration {
	configID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	tenantID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	scheduleJSON, _ := json.Marshal(map[string]string{
		"cron_expression": "0 8 * * MON",
		"timezone":        "America/New_York",
	})
	return &PlanConfiguration{
		ID:       configID,
		TenantID: tenantID,
		Status:   ConfigurationStatusScheduled,
		Schedule: scheduleJSON,
	}
}

func TestValidateCronExpression(t *testing.T) {
	if err := validateCronExpression("0 8 * * MON"); err != nil {
		t.Fatalf("expected valid cron, got %v", err)
	}
	if err := validateCronExpression("not-a-cron"); err == nil {
		t.Fatal("expected invalid cron error")
	}
}

func TestValidateScheduleForStatusRequiresCron(t *testing.T) {
	err := validateScheduleForStatus(ConfigurationStatusScheduled, json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected cron required error")
	}
}

func TestScheduleManagerNilTemporalNoOp(t *testing.T) {
	manager := NewScheduleManager(nil, slog.Default())
	if err := manager.Sync(context.Background(), scheduledConfig()); err != nil {
		t.Fatalf("Sync() = %v, want nil", err)
	}
}

func TestScheduleManagerCreatesScheduleWhenScheduled(t *testing.T) {
	mockClient := &mockScheduleClient{}
	manager := &ScheduleManager{
		schedules: mockClient,
		logger:    slog.Default(),
	}

	if err := manager.Sync(context.Background(), scheduledConfig()); err != nil {
		t.Fatalf("Sync() = %v", err)
	}
	if mockClient.createCalls != 1 {
		t.Fatalf("createCalls = %d, want 1", mockClient.createCalls)
	}
}

func TestScheduleManagerUpdatesExistingSchedule(t *testing.T) {
	mockClient := &mockScheduleClient{}
	scheduleID := scheduleIDForConfig(uuid.MustParse("11111111-1111-1111-1111-111111111111"))
	mockClient.handles = map[string]*mockScheduleHandle{
		scheduleID: {id: scheduleID},
	}

	manager := &ScheduleManager{
		schedules: mockClient,
		logger:    slog.Default(),
	}

	if err := manager.Sync(context.Background(), scheduledConfig()); err != nil {
		t.Fatalf("Sync() = %v", err)
	}
	if mockClient.createCalls != 0 {
		t.Fatalf("createCalls = %d, want 0", mockClient.createCalls)
	}
	if !mockClient.handles[scheduleID].updated {
		t.Fatal("expected schedule update")
	}
}

func TestScheduleManagerPausesWhenDisabled(t *testing.T) {
	mockClient := &mockScheduleClient{}
	config := scheduledConfig()
	config.Status = ConfigurationStatusDisabled
	scheduleID := scheduleIDForConfig(config.ID)
	mockClient.handles = map[string]*mockScheduleHandle{
		scheduleID: {id: scheduleID},
	}

	manager := &ScheduleManager{
		schedules: mockClient,
		logger:    slog.Default(),
	}

	if err := manager.Sync(context.Background(), config); err != nil {
		t.Fatalf("Sync() = %v", err)
	}
	if !mockClient.handles[scheduleID].paused {
		t.Fatal("expected schedule pause")
	}
	if mockClient.handles[scheduleID].deleted {
		t.Fatal("did not expect schedule delete")
	}
}

func TestScheduleManagerDeletesWhenNotScheduled(t *testing.T) {
	mockClient := &mockScheduleClient{}
	config := scheduledConfig()
	config.Status = ConfigurationStatusRunnable
	scheduleID := scheduleIDForConfig(config.ID)
	mockClient.handles = map[string]*mockScheduleHandle{
		scheduleID: {id: scheduleID},
	}

	manager := &ScheduleManager{
		schedules: mockClient,
		logger:    slog.Default(),
	}

	if err := manager.Sync(context.Background(), config); err != nil {
		t.Fatalf("Sync() = %v", err)
	}
	if !mockClient.handles[scheduleID].deleted {
		t.Fatal("expected schedule delete")
	}
}

func TestBuildScheduleActionUsesPlanWorkflow(t *testing.T) {
	config := scheduledConfig()
	action := buildScheduleAction(config)
	if action.Workflow != workflow.PlanScheduledExecutionWorkflowName {
		t.Fatalf("workflow = %v, want %s", action.Workflow, workflow.PlanScheduledExecutionWorkflowName)
	}
	if action.TaskQueue != workflow.TaskQueueName {
		t.Fatalf("task queue = %q, want %q", action.TaskQueue, workflow.TaskQueueName)
	}
}
