package artifacts

import (
	"context"
	"sort"
	"strconv"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	artifactsv1 "github.com/harpia/control-plane/gen/harpia/artifacts/v1"
	"github.com/harpia/control-plane/internal/identity"
)

type mockRepository struct {
	artifacts map[uuid.UUID]*Artifact
	types     map[uuid.UUID]*ArtifactType
	versions  map[uuid.UUID][]*ArtifactVersion
}

func (m *mockRepository) RegisterType(_ context.Context, artifactType *ArtifactType) (*ArtifactType, error) {
	if m.types == nil {
		m.types = make(map[uuid.UUID]*ArtifactType)
	}
	created := *artifactType
	if created.ID == uuid.Nil {
		created.ID = uuid.New()
	}
	m.types[created.ID] = &created
	return &created, nil
}

func (m *mockRepository) GetTypeByID(_ context.Context, typeID uuid.UUID) (*ArtifactType, error) {
	artifactType, ok := m.types[typeID]
	if !ok {
		return nil, pgx.ErrNoRows
	}
	return artifactType, nil
}

func (m *mockRepository) GetTypeByKey(_ context.Context, key string) (*ArtifactType, error) {
	for _, artifactType := range m.types {
		if artifactType.Key == key {
			return artifactType, nil
		}
	}
	return nil, pgx.ErrNoRows
}

func (m *mockRepository) CreateArtifact(_ context.Context, artifact *Artifact) (*Artifact, error) {
	if m.artifacts == nil {
		m.artifacts = make(map[uuid.UUID]*Artifact)
	}
	created := *artifact
	if created.CreatedAt.IsZero() {
		created.CreatedAt = time.Now().UTC()
	}
	m.artifacts[created.ID] = &created
	return &created, nil
}

func (m *mockRepository) GetArtifact(_ context.Context, tenantID, artifactID uuid.UUID) (*Artifact, error) {
	artifact, ok := m.artifacts[artifactID]
	if !ok || artifact.TenantID != tenantID {
		return nil, pgx.ErrNoRows
	}
	return artifact, nil
}

func (m *mockRepository) ListArtifacts(_ context.Context, tenantID uuid.UUID, filter ArtifactListFilter) ([]Artifact, error) {
	var result []Artifact
	for _, artifact := range m.artifacts {
		if artifact.TenantID != tenantID {
			continue
		}
		if filter.PlanConfigurationID != nil &&
			(!artifact.PlanConfigurationID.Valid || artifact.PlanConfigurationID.UUID != *filter.PlanConfigurationID) {
			continue
		}
		if filter.PlanExecutionID != nil &&
			(!artifact.PlanExecutionID.Valid || artifact.PlanExecutionID.UUID != *filter.PlanExecutionID) {
			continue
		}
		if filter.ArtifactTypeKey != "" {
			typeKey := artifact.ArtifactTypeKey
			if typeKey == "" {
				if artifactType, ok := m.types[artifact.ArtifactTypeID]; ok {
					typeKey = artifactType.Key
				}
			}
			if typeKey != filter.ArtifactTypeKey {
				continue
			}
		}
		result = append(result, *artifact)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})
	start := min(filter.Offset, len(result))
	end := min(start+filter.Limit, len(result))
	return result[start:end], nil
}

func (m *mockRepository) CreateArtifactVersion(_ context.Context, artifact *Artifact, version *ArtifactVersion) (*ArtifactVersion, *Artifact, error) {
	if m.artifacts == nil {
		m.artifacts = make(map[uuid.UUID]*Artifact)
	}
	if m.versions == nil {
		m.versions = make(map[uuid.UUID][]*ArtifactVersion)
	}
	current, ok := m.artifacts[artifact.ID]
	if !ok || current.TenantID != artifact.TenantID {
		return nil, nil, pgx.ErrNoRows
	}

	created := *version
	if created.ID == uuid.Nil {
		created.ID = uuid.New()
	}
	created.ArtifactID = artifact.ID
	created.TenantID = artifact.TenantID
	if created.VersionNumber == 0 {
		created.VersionNumber = int32(len(m.versions[artifact.ID]) + 1)
	}
	if created.CreatedAt.IsZero() {
		created.CreatedAt = time.Now().UTC()
	}
	m.versions[artifact.ID] = append(m.versions[artifact.ID], &created)

	updated := *current
	updated.StorageURI = created.StorageURI
	updated.ContentHash = created.ContentHash
	updated.CurrentVersionID = uuid.NullUUID{UUID: created.ID, Valid: true}
	updated.Status = artifactsv1.ArtifactStatus_ARTIFACT_STATUS_EDITED
	updated.UpdatedAt = created.CreatedAt
	m.artifacts[artifact.ID] = &updated
	return &created, &updated, nil
}

func (m *mockRepository) ListArtifactVersions(_ context.Context, tenantID, artifactID uuid.UUID, limit, offset int) ([]ArtifactVersion, string, error) {
	artifact, ok := m.artifacts[artifactID]
	if !ok || artifact.TenantID != tenantID {
		return nil, "", pgx.ErrNoRows
	}

	versions := append([]*ArtifactVersion(nil), m.versions[artifactID]...)
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].VersionNumber > versions[j].VersionNumber
	})

	start := min(offset, len(versions))
	end := min(start+limit, len(versions))
	result := make([]ArtifactVersion, 0, end-start)
	for _, version := range versions[start:end] {
		result = append(result, *version)
	}

	nextToken := ""
	if end < len(versions) {
		nextToken = strconv.Itoa(end)
	}
	return result, nextToken, nil
}

func (m *mockRepository) GetArtifactVersion(_ context.Context, tenantID, artifactID, versionID uuid.UUID) (*ArtifactVersion, error) {
	artifact, ok := m.artifacts[artifactID]
	if !ok || artifact.TenantID != tenantID {
		return nil, pgx.ErrNoRows
	}
	for _, version := range m.versions[artifactID] {
		if version.ID == versionID && version.TenantID == tenantID {
			return version, nil
		}
	}
	return nil, pgx.ErrNoRows
}

func tenantContext(tenantID uuid.UUID) context.Context {
	return identity.WithRequestContext(context.Background(), identity.RequestContext{
		TenantID: tenantID,
		UserID:   "test-user",
	})
}

func TestPreviewArtifactTextDraft(t *testing.T) {
	tenantID := uuid.New()
	typeID := uuid.New()
	artifactID := uuid.New()
	storageURI := formatStorageURI("harpia", "tenant/test/artifact.json")
	payload := []byte(`{"title":"Draft","body":"Preview body"}`)

	repo := &mockRepository{
		types: map[uuid.UUID]*ArtifactType{
			typeID: {ID: typeID, Key: TypeKeyTextDraft},
		},
		artifacts: map[uuid.UUID]*Artifact{
			artifactID: {
				ID:             artifactID,
				TenantID:       tenantID,
				ArtifactTypeID: typeID,
				StorageURI:     storageURI,
			},
		},
	}
	store := &mockPayloadStore{objects: map[string][]byte{storageURI: payload}}
	handler := &Handler{repo: repo, store: store}

	resp, err := handler.PreviewArtifact(
		tenantContext(tenantID),
		connect.NewRequest(&artifactsv1.PreviewArtifactRequest{
			TenantId:   tenantID.String(),
			ArtifactId: artifactID.String(),
		}),
	)
	if err != nil {
		t.Fatalf("PreviewArtifact: %v", err)
	}
	if resp.Msg.GetMarkdownPreview() != "# Draft\n\nPreview body" {
		t.Fatalf("markdown preview = %q", resp.Msg.GetMarkdownPreview())
	}
}

func TestPreviewArtifactNewsList(t *testing.T) {
	tenantID := uuid.New()
	typeID := uuid.New()
	artifactID := uuid.New()
	storageURI := formatStorageURI("harpia", "tenant/test/news.json")
	payload := []byte(`{"articles":[{"title":"Headline","url":"https://example.com"}]}`)

	repo := &mockRepository{
		types: map[uuid.UUID]*ArtifactType{
			typeID: {ID: typeID, Key: TypeKeyNewsList},
		},
		artifacts: map[uuid.UUID]*Artifact{
			artifactID: {
				ID:             artifactID,
				TenantID:       tenantID,
				ArtifactTypeID: typeID,
				StorageURI:     storageURI,
			},
		},
	}
	store := &mockPayloadStore{objects: map[string][]byte{storageURI: payload}}
	handler := &Handler{repo: repo, store: store}

	resp, err := handler.PreviewArtifact(
		tenantContext(tenantID),
		connect.NewRequest(&artifactsv1.PreviewArtifactRequest{
			TenantId:   tenantID.String(),
			ArtifactId: artifactID.String(),
		}),
	)
	if err != nil {
		t.Fatalf("PreviewArtifact: %v", err)
	}

	summary := resp.Msg.GetListSummary()
	if summary == nil || summary.ArticleCount != 1 || summary.Titles[0] != "Headline" {
		t.Fatalf("list summary = %#v", summary)
	}
}

func TestPreviewArtifactCrossTenantDenied(t *testing.T) {
	ownerTenant := uuid.New()
	otherTenant := uuid.New()
	typeID := uuid.New()
	artifactID := uuid.New()

	repo := &mockRepository{
		types: map[uuid.UUID]*ArtifactType{
			typeID: {ID: typeID, Key: TypeKeyTextDraft},
		},
		artifacts: map[uuid.UUID]*Artifact{
			artifactID: {
				ID:             artifactID,
				TenantID:       ownerTenant,
				ArtifactTypeID: typeID,
				StorageURI:     formatStorageURI("harpia", "tenant/owner/artifact.json"),
			},
		},
	}
	handler := &Handler{repo: repo, store: &mockPayloadStore{}}

	_, err := handler.PreviewArtifact(
		tenantContext(otherTenant),
		connect.NewRequest(&artifactsv1.PreviewArtifactRequest{
			TenantId:   ownerTenant.String(),
			ArtifactId: artifactID.String(),
		}),
	)
	if err == nil || connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("expected permission denied, got %v", err)
	}
}

func TestPreviewArtifactNotFoundForOtherTenantArtifact(t *testing.T) {
	ownerTenant := uuid.New()
	callerTenant := uuid.New()
	typeID := uuid.New()
	artifactID := uuid.New()

	repo := &mockRepository{
		types: map[uuid.UUID]*ArtifactType{
			typeID: {ID: typeID, Key: TypeKeyTextDraft},
		},
		artifacts: map[uuid.UUID]*Artifact{
			artifactID: {
				ID:             artifactID,
				TenantID:       ownerTenant,
				ArtifactTypeID: typeID,
				StorageURI:     formatStorageURI("harpia", "tenant/owner/artifact.json"),
			},
		},
	}
	handler := &Handler{repo: repo, store: &mockPayloadStore{}}

	_, err := handler.PreviewArtifact(
		tenantContext(callerTenant),
		connect.NewRequest(&artifactsv1.PreviewArtifactRequest{
			TenantId:   callerTenant.String(),
			ArtifactId: artifactID.String(),
		}),
	)
	if err == nil || connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestPreviewArtifactMissingTenantContext(t *testing.T) {
	handler := &Handler{repo: &mockRepository{}, store: &mockPayloadStore{}}
	_, err := handler.PreviewArtifact(
		context.Background(),
		connect.NewRequest(&artifactsv1.PreviewArtifactRequest{
			TenantId:   uuid.NewString(),
			ArtifactId: uuid.NewString(),
		}),
	)
	if err == nil || connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("expected unauthenticated, got %v", err)
	}
}
