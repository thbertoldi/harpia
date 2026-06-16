package contracttest

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"

	artifactsv1 "github.com/harpia/control-plane/gen/harpia/artifacts/v1"
	"github.com/harpia/control-plane/internal/artifacts"
	"github.com/harpia/control-plane/internal/executors/runtime"
)

// Suite validates the shared integration executor contract for one SKU handler.
type Suite struct {
	Name string

	Handler runtime.IntegrationHandler
	Store   runtime.ExecutorArtifactStore

	SKUKey                string
	InputArtifactTypeKey  string
	OutputArtifactTypeKey string

	ValidInstallationConfig json.RawMessage
	ValidInputLiteralJSON   string

	RetryableTrigger func(ctx context.Context, req runtime.IntegrationExecutionRequest) error
}

// Run executes the standard integration contract checks.
func Run(t *testing.T, suite Suite) {
	t.Helper()
	if suite.Handler == nil || suite.Store == nil {
		t.Fatal("contract suite requires Handler and Store")
	}
	if suite.Name == "" {
		suite.Name = suite.Handler.SKUKey()
	}

	t.Run(suite.Name+"/loads-required-input-artifact", func(t *testing.T) {
		suite.assertLoadsRequiredInput(t)
	})
	t.Run(suite.Name+"/resolves-output-artifact-type-key", func(t *testing.T) {
		suite.assertResolvesOutputTypeKey(t)
	})
	t.Run(suite.Name+"/validates-schema-before-create", func(t *testing.T) {
		suite.assertSchemaValidationBeforeCreate(t)
	})
	t.Run(suite.Name+"/terminal-invalid-config", func(t *testing.T) {
		suite.assertTerminalInvalidConfig(t)
	})
	if suite.RetryableTrigger != nil {
		t.Run(suite.Name+"/maps-retryable-error", func(t *testing.T) {
			suite.assertRetryableErrorMapping(t)
		})
	}
}

func (s Suite) baseRequest() runtime.IntegrationExecutionRequest {
	return runtime.IntegrationExecutionRequest{
		TenantID:              uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		StepExecutionID:       "step-contract-test",
		OutputArtifactTypeKey: s.OutputArtifactTypeKey,
		InputArtifacts: []runtime.InputArtifactRef{{
			ArtifactTypeKey: s.InputArtifactTypeKey,
			LiteralJSON:     s.ValidInputLiteralJSON,
		}},
		Installation: runtime.InstallationSnapshot{
			ExecutorSKUKey: s.SKUKey,
			ConfigJSON:     s.ValidInstallationConfig,
		},
	}
}

func (s Suite) assertLoadsRequiredInput(t *testing.T) {
	t.Helper()
	req := s.baseRequest()
	req.InputArtifacts = nil

	result, err := s.Handler.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Status != runtime.IntegrationStatusFailed {
		t.Fatalf("Status = %q, want failed when input artifact is missing", result.Status)
	}
	if result.Error == "" {
		t.Fatal("expected terminal failure message for missing input artifact")
	}
}

func (s Suite) assertResolvesOutputTypeKey(t *testing.T) {
	t.Helper()
	req := s.baseRequest()
	if req.OutputArtifactTypeKey == "" {
		t.Skip("output artifact type key not configured for suite")
	}

	result, err := s.Handler.Execute(context.Background(), req)
	if err != nil {
		if _, ok := runtime.IsRetryable(err); ok {
			t.Skip("retryable failure during output resolution smoke test")
		}
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Status != runtime.IntegrationStatusCompleted {
		t.Fatalf("Status = %q, want completed (%s)", result.Status, result.Error)
	}
	if result.OutputArtifactID == "" {
		t.Fatal("expected output artifact id when handler completes")
	}
}

func (s Suite) assertSchemaValidationBeforeCreate(t *testing.T) {
	t.Helper()
	_, err := s.Store.CreateValidatedPayload(context.Background(), runtime.CreateArtifactRequest{
		TenantID:              uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		OutputArtifactTypeKey: s.OutputArtifactTypeKey,
		StepExecutionID:       "step-contract-test",
		Payload:               []byte(`{"not":"a-valid-news-list"}`),
	})
	if err == nil {
		t.Fatal("expected schema validation error for invalid payload")
	}
}

func (s Suite) assertTerminalInvalidConfig(t *testing.T) {
	t.Helper()
	req := s.baseRequest()
	req.Installation.ConfigJSON = json.RawMessage(`{}`)

	result, err := s.Handler.Execute(context.Background(), req)
	if err != nil {
		if _, ok := runtime.IsRetryable(err); ok {
			t.Fatal("invalid config must not produce retryable error")
		}
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Status != runtime.IntegrationStatusFailed {
		t.Fatalf("Status = %q, want failed for invalid config", result.Status)
	}
}

func (s Suite) assertRetryableErrorMapping(t *testing.T) {
	t.Helper()
	req := s.baseRequest()
	err := s.RetryableTrigger(context.Background(), req)
	if err == nil {
		t.Fatal("expected retryable trigger to produce an error")
	}
	retryable, ok := runtime.IsRetryable(err)
	if !ok {
		t.Fatalf("error = %T(%v), want *runtime.RetryableError", err, err)
	}
	if retryable.Code == "" {
		t.Fatal("retryable error code is required")
	}
}

// MemoryArtifactRepo is a minimal in-memory ArtifactRepository for contract tests.
type MemoryArtifactRepo struct {
	Types     map[string]*artifacts.ArtifactType
	Artifacts map[uuid.UUID]*artifacts.Artifact
}

func (m *MemoryArtifactRepo) GetTypeByID(_ context.Context, typeID uuid.UUID) (*artifacts.ArtifactType, error) {
	for _, artifactType := range m.Types {
		if artifactType.ID == typeID {
			return artifactType, nil
		}
	}
	return nil, context.Canceled
}

func (m *MemoryArtifactRepo) GetTypeByKey(_ context.Context, key string) (*artifacts.ArtifactType, error) {
	artifactType, ok := m.Types[key]
	if !ok {
		return nil, context.Canceled
	}
	return artifactType, nil
}

func (m *MemoryArtifactRepo) CreateArtifact(_ context.Context, artifact *artifacts.Artifact) (*artifacts.Artifact, error) {
	if m.Artifacts == nil {
		m.Artifacts = make(map[uuid.UUID]*artifacts.Artifact)
	}
	created := *artifact
	if created.ID == uuid.Nil {
		created.ID = uuid.New()
	}
	if created.CreatedAt.IsZero() {
		created.CreatedAt = time.Now().UTC()
	}
	m.Artifacts[created.ID] = &created
	return &created, nil
}

func (m *MemoryArtifactRepo) GetArtifact(_ context.Context, tenantID, artifactID uuid.UUID) (*artifacts.Artifact, error) {
	artifact, ok := m.Artifacts[artifactID]
	if !ok || artifact.TenantID != tenantID {
		return nil, context.Canceled
	}
	return artifact, nil
}

// MemoryPayloadStore is a minimal in-memory PayloadStore for contract tests.
type MemoryPayloadStore struct {
	Objects map[string][]byte
}

func (m *MemoryPayloadStore) Put(_ context.Context, objectPath string, payload []byte) (string, error) {
	if m.Objects == nil {
		m.Objects = make(map[string][]byte)
	}
	uri := "s3://harpia/tenant/test/" + objectPath
	m.Objects[uri] = append([]byte(nil), payload...)
	return uri, nil
}

func (m *MemoryPayloadStore) Get(_ context.Context, storageURI string) ([]byte, error) {
	payload, ok := m.Objects[storageURI]
	if !ok {
		return nil, context.Canceled
	}
	return append([]byte(nil), payload...), nil
}

// NewRSSArtifactStore returns an artifact store seeded for RSS contract tests.
func NewRSSArtifactStore() runtime.ExecutorArtifactStore {
	typeID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	return runtime.NewExecutorArtifactStore(&MemoryArtifactRepo{
		Types: map[string]*artifacts.ArtifactType{
			artifacts.TypeKeyNewsList: {ID: typeID, Key: artifacts.TypeKeyNewsList},
		},
	}, &MemoryPayloadStore{})
}

// ValidRSSDateRangeLiteral is a schema-valid DateRange literal for contract tests.
const ValidRSSDateRangeLiteral = `{"startDate":"2026-01-01","endDate":"2026-01-07"}`

// AssertNewsListPayload validates a stored NewsList payload against the schema.
func AssertNewsListPayload(t *testing.T, payload []byte) {
	t.Helper()
	newsList := &artifactsv1.NewsList{}
	if err := protojson.Unmarshal(payload, newsList); err != nil {
		t.Fatalf("unmarshal news list: %v", err)
	}
	if err := artifacts.ValidatePayload(artifacts.TypeKeyNewsList, payload); err != nil {
		t.Fatalf("news list schema validation: %v", err)
	}
}

// IsRetryable is a test helper alias for clarity in integration-specific tests.
func IsRetryable(err error) (*runtime.RetryableError, bool) {
	return runtime.IsRetryable(err)
}

// RequireRetryable fails the test unless err is a RetryableError.
func RequireRetryable(t *testing.T, err error) {
	t.Helper()
	if _, ok := IsRetryable(err); !ok {
		t.Fatalf("error = %v, want retryable", err)
	}
}

// RequireTerminalFailure fails unless result is a failed integration result without error.
func RequireTerminalFailure(t *testing.T, result runtime.IntegrationExecutionResult, err error) {
	t.Helper()
	if err != nil {
		if _, ok := runtime.IsRetryable(err); ok {
			t.Fatalf("expected terminal failure, got retryable error: %v", err)
		}
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Status != runtime.IntegrationStatusFailed {
		t.Fatalf("Status = %q, want failed", result.Status)
	}
	if result.Error == "" {
		t.Fatal("expected failure message")
	}
}

// Unused import guard removed — contract helpers use runtime directly.
