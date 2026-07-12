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
	Versions  map[uuid.UUID]*artifacts.ArtifactVersion
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
	if m.Versions == nil {
		m.Versions = make(map[uuid.UUID]*artifacts.ArtifactVersion)
	}
	versionID := uuid.New()
	created.CurrentVersionID = uuid.NullUUID{UUID: versionID, Valid: true}
	m.Artifacts[created.ID] = &created
	m.Versions[versionID] = &artifacts.ArtifactVersion{
		ID: versionID, ArtifactID: created.ID, TenantID: created.TenantID,
		VersionNumber: 1, StorageURI: created.StorageURI, ContentHash: created.ContentHash,
	}
	return &created, nil
}

func (m *MemoryArtifactRepo) CreateArtifactVersion(_ context.Context, artifact *artifacts.Artifact, version *artifacts.ArtifactVersion) (*artifacts.ArtifactVersion, *artifacts.Artifact, error) {
	current, ok := m.Artifacts[artifact.ID]
	if !ok || current.TenantID != artifact.TenantID || current.ContentHash != artifact.ContentHash {
		return nil, nil, context.Canceled
	}
	if m.Versions == nil {
		m.Versions = make(map[uuid.UUID]*artifacts.ArtifactVersion)
	}
	created := *version
	if created.ID == uuid.Nil {
		created.ID = uuid.New()
	}
	created.ArtifactID = current.ID
	created.TenantID = current.TenantID
	created.VersionNumber = int32(len(m.Versions) + 1)
	m.Versions[created.ID] = &created
	updated := *current
	updated.StorageURI = created.StorageURI
	updated.ContentHash = created.ContentHash
	updated.CurrentVersionID = uuid.NullUUID{UUID: created.ID, Valid: true}
	m.Artifacts[updated.ID] = &updated
	return &created, &updated, nil
}

func (m *MemoryArtifactRepo) GetArtifact(_ context.Context, tenantID, artifactID uuid.UUID) (*artifacts.Artifact, error) {
	artifact, ok := m.Artifacts[artifactID]
	if !ok || artifact.TenantID != tenantID {
		return nil, context.Canceled
	}
	return artifact, nil
}

func (m *MemoryArtifactRepo) GetArtifactVersion(_ context.Context, tenantID, artifactID, versionID uuid.UUID) (*artifacts.ArtifactVersion, error) {
	if v, ok := m.Versions[versionID]; ok && v.TenantID == tenantID && v.ArtifactID == artifactID {
		return v, nil
	}
	return nil, context.Canceled
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

// NewLinkedInArtifactStore returns an artifact store seeded for LinkedIn contract tests.
func NewLinkedInArtifactStore() runtime.ExecutorArtifactStore {
	typeID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	return runtime.NewExecutorArtifactStore(&MemoryArtifactRepo{
		Types: map[string]*artifacts.ArtifactType{
			artifacts.TypeKeyPublishConfirmation: {ID: typeID, Key: artifacts.TypeKeyPublishConfirmation},
			artifacts.TypeKeyLinkedInPost:        {ID: uuid.MustParse("55555555-5555-5555-5555-555555555556"), Key: artifacts.TypeKeyLinkedInPost},
		},
	}, &MemoryPayloadStore{})
}

// ValidLinkedInPostDraftLiteral is a schema-valid LinkedInPostDraft literal for contract tests.
const ValidLinkedInPostDraftLiteral = `{"text":"Shipping LinkedIn publish integration.","hashtags":["harpia","automation"]}`

const ValidLinkedInPostLiteral = `{"text":{"text":"Shipping LinkedIn publish integration.","hashtags":["harpia","automation"]}}`

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

// AssertPublishConfirmationPayload validates a stored PublishConfirmation payload.
func AssertPublishConfirmationPayload(t *testing.T, payload []byte) {
	t.Helper()
	confirmation := &artifactsv1.PublishConfirmation{}
	if err := protojson.Unmarshal(payload, confirmation); err != nil {
		t.Fatalf("unmarshal publish confirmation: %v", err)
	}
	if err := artifacts.ValidatePayload(artifacts.TypeKeyPublishConfirmation, payload); err != nil {
		t.Fatalf("publish confirmation schema validation: %v", err)
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
