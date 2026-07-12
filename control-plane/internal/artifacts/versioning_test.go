package artifacts

import (
	"context"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	artifactsv1 "github.com/harpia/control-plane/gen/harpia/artifacts/v1"
	"github.com/harpia/control-plane/internal/identity"
)

func tenantUserContext(tenantID, userID uuid.UUID) context.Context {
	return identity.WithRequestContext(context.Background(), identity.RequestContext{
		TenantID: tenantID,
		UserID:   userID.String(),
	})
}

func TestSaveTextArtifactVersionCreatesVersionAndUpdatesCurrent(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	typeID := uuid.New()
	artifactID := uuid.New()
	versionID := uuid.New()
	currentURI := formatStorageURI("harpia", "tenant/test/current.json")
	currentPayload := []byte(`{"text":"Original post","hook":"Original","hashtags":["growth"]}`)
	currentHash := ContentHash(currentPayload)

	repo := &mockRepository{
		types: map[uuid.UUID]*ArtifactType{
			typeID: {ID: typeID, Key: TypeKeyLinkedInPostDraft},
		},
		artifacts: map[uuid.UUID]*Artifact{
			artifactID: {
				ID:               artifactID,
				TenantID:         tenantID,
				ArtifactTypeID:   typeID,
				ArtifactTypeKey:  TypeKeyLinkedInPostDraft,
				StorageURI:       currentURI,
				ContentHash:      currentHash,
				CurrentVersionID: uuid.NullUUID{UUID: versionID, Valid: true},
				Status:           artifactsv1.ArtifactStatus_ARTIFACT_STATUS_GENERATED,
				CreatedAt:        time.Now().Add(-time.Hour).UTC(),
				UpdatedAt:        time.Now().Add(-time.Hour).UTC(),
			},
		},
		versions: map[uuid.UUID][]*ArtifactVersion{
			artifactID: {{
				ID:            versionID,
				ArtifactID:    artifactID,
				TenantID:      tenantID,
				VersionNumber: 1,
				StorageURI:    currentURI,
				ContentHash:   currentHash,
				CreatedByKind: artifactsv1.ArtifactVersionCreatedByKind_ARTIFACT_VERSION_CREATED_BY_KIND_AGENT,
			}},
		},
	}
	store := &mockPayloadStore{objects: map[string][]byte{currentURI: currentPayload}}
	handler := &Handler{repo: repo, store: store}

	resp, err := handler.SaveTextArtifactVersion(
		tenantUserContext(tenantID, userID),
		connect.NewRequest(&artifactsv1.SaveTextArtifactVersionRequest{
			TenantId:            tenantID.String(),
			ArtifactId:          artifactID.String(),
			ExpectedContentHash: currentHash,
			Title:               "Edited hook",
			Text:                "Original post\n\nAdded line.",
			EditSummary:         "Added CTA line",
		}),
	)
	if err != nil {
		t.Fatalf("SaveTextArtifactVersion: %v", err)
	}
	if resp.Msg.Version.GetVersionNumber() != 2 {
		t.Fatalf("version = %d, want 2", resp.Msg.Version.GetVersionNumber())
	}
	if resp.Msg.Artifact.GetContentHash() == currentHash {
		t.Fatal("content hash did not change")
	}
	if resp.Msg.Version.GetCreatedByKind() != artifactsv1.ArtifactVersionCreatedByKind_ARTIFACT_VERSION_CREATED_BY_KIND_USER {
		t.Fatalf("created_by_kind = %v, want user", resp.Msg.Version.GetCreatedByKind())
	}
	if resp.Msg.Version.GetEditSummary() != "Added CTA line" {
		t.Fatalf("edit_summary = %q", resp.Msg.Version.GetEditSummary())
	}
	if len(store.objects) != 2 {
		t.Fatalf("stored objects = %d, want 2", len(store.objects))
	}
	edited := store.objects[resp.Msg.Artifact.GetStorageUri()]
	if string(edited) == string(currentPayload) {
		t.Fatal("stored edited payload did not change")
	}
	if !strings.Contains(string(edited), "Edited hook") {
		t.Fatalf("stored edited payload = %s, want edited hook", string(edited))
	}
}

func TestSaveTextArtifactVersionRejectsHashConflict(t *testing.T) {
	tenantID := uuid.New()
	typeID := uuid.New()
	artifactID := uuid.New()
	currentURI := formatStorageURI("harpia", "tenant/test/current.json")
	currentPayload := []byte(`{"title":"Draft","body":"Original body"}`)
	currentHash := ContentHash(currentPayload)

	repo := &mockRepository{
		types: map[uuid.UUID]*ArtifactType{
			typeID: {ID: typeID, Key: TypeKeyTextDraft},
		},
		artifacts: map[uuid.UUID]*Artifact{
			artifactID: {
				ID:              artifactID,
				TenantID:        tenantID,
				ArtifactTypeID:  typeID,
				ArtifactTypeKey: TypeKeyTextDraft,
				StorageURI:      currentURI,
				ContentHash:     currentHash,
			},
		},
	}
	store := &mockPayloadStore{objects: map[string][]byte{currentURI: currentPayload}}
	handler := &Handler{repo: repo, store: store}

	_, err := handler.SaveTextArtifactVersion(
		tenantContext(tenantID),
		connect.NewRequest(&artifactsv1.SaveTextArtifactVersionRequest{
			TenantId:            tenantID.String(),
			ArtifactId:          artifactID.String(),
			ExpectedContentHash: "stale",
			Text:                "Edited body",
		}),
	)
	if err == nil || connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("expected failed precondition, got %v", err)
	}
	if len(store.objects) != 1 {
		t.Fatalf("stored objects = %d, want 1", len(store.objects))
	}
}

func TestPreviewArtifactUsesRequestedVersion(t *testing.T) {
	tenantID := uuid.New()
	typeID := uuid.New()
	artifactID := uuid.New()
	currentURI := formatStorageURI("harpia", "tenant/test/current.json")
	oldURI := formatStorageURI("harpia", "tenant/test/old.json")
	oldVersionID := uuid.New()

	repo := &mockRepository{
		types: map[uuid.UUID]*ArtifactType{
			typeID: {ID: typeID, Key: TypeKeyTextDraft},
		},
		artifacts: map[uuid.UUID]*Artifact{
			artifactID: {
				ID:              artifactID,
				TenantID:        tenantID,
				ArtifactTypeID:  typeID,
				ArtifactTypeKey: TypeKeyTextDraft,
				StorageURI:      currentURI,
				ContentHash:     ContentHash([]byte(`{"title":"Current","body":"Current body"}`)),
			},
		},
		versions: map[uuid.UUID][]*ArtifactVersion{
			artifactID: {{
				ID:            oldVersionID,
				ArtifactID:    artifactID,
				TenantID:      tenantID,
				VersionNumber: 1,
				StorageURI:    oldURI,
				ContentHash:   ContentHash([]byte(`{"title":"Old","body":"Old body"}`)),
			}},
		},
	}
	store := &mockPayloadStore{objects: map[string][]byte{
		currentURI: []byte(`{"title":"Current","body":"Current body"}`),
		oldURI:     []byte(`{"title":"Old","body":"Old body"}`),
	}}
	handler := &Handler{repo: repo, store: store}

	resp, err := handler.PreviewArtifact(
		tenantContext(tenantID),
		connect.NewRequest(&artifactsv1.PreviewArtifactRequest{
			TenantId:          tenantID.String(),
			ArtifactId:        artifactID.String(),
			ArtifactVersionId: ptrString(oldVersionID.String()),
		}),
	)
	if err != nil {
		t.Fatalf("PreviewArtifact: %v", err)
	}
	if got := resp.Msg.GetMarkdownPreview(); got != "# Old\n\nOld body" {
		t.Fatalf("markdown preview = %q, want old version preview", got)
	}
}

func TestCreateArtifactVersionWithPayloadCreatesNamedVersion(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	typeID := uuid.New()
	artifactID := uuid.New()
	versionID := uuid.New()
	currentURI := formatStorageURI("harpia", "tenant/test/post-current.json")
	currentPayload := []byte(`{"text":{"text":"Original"}}`)
	currentHash := ContentHash(currentPayload)
	repo := &mockRepository{
		types: map[uuid.UUID]*ArtifactType{
			typeID: {ID: typeID, Key: TypeKeyLinkedInPost},
		},
		artifacts: map[uuid.UUID]*Artifact{
			artifactID: {
				ID: artifactID, TenantID: tenantID, ArtifactTypeID: typeID,
				ArtifactTypeKey: TypeKeyLinkedInPost, StorageURI: currentURI,
				ContentHash: currentHash, CurrentVersionID: uuid.NullUUID{UUID: versionID, Valid: true},
			},
		},
		versions: map[uuid.UUID][]*ArtifactVersion{
			artifactID: {{ID: versionID, ArtifactID: artifactID, TenantID: tenantID, VersionNumber: 1, StorageURI: currentURI, ContentHash: currentHash}},
		},
	}
	store := &mockPayloadStore{objects: map[string][]byte{currentURI: currentPayload}}
	handler := &Handler{repo: repo, store: store}
	payload := []byte(`{"text":{"text":"Revised"}}`)
	resp, err := handler.CreateArtifactVersionWithPayload(
		tenantUserContext(tenantID, userID),
		connect.NewRequest(&artifactsv1.CreateArtifactVersionWithPayloadRequest{
			TenantId: tenantID.String(), ArtifactId: artifactID.String(), ExpectedContentHash: currentHash,
			PayloadJson: payload, EditSummary: "Review revision",
		}),
	)
	if err != nil {
		t.Fatalf("CreateArtifactVersionWithPayload: %v", err)
	}
	if resp.Msg.GetArtifactVersion().GetVersionNumber() != 2 {
		t.Fatalf("version number = %d, want 2", resp.Msg.GetArtifactVersion().GetVersionNumber())
	}
	if resp.Msg.GetArtifact().GetContentHash() != ContentHash(payload) {
		t.Fatalf("artifact hash = %q, want payload hash", resp.Msg.GetArtifact().GetContentHash())
	}
}

func TestListArtifactsFiltersByPlanExecution(t *testing.T) {
	tenantID := uuid.New()
	typeID := uuid.New()
	wantedExecutionID := uuid.New()
	otherExecutionID := uuid.New()
	repo := &mockRepository{
		types: map[uuid.UUID]*ArtifactType{
			typeID: {ID: typeID, Key: TypeKeyTextDraft},
		},
		artifacts: map[uuid.UUID]*Artifact{
			uuid.New(): {
				ID:              uuid.New(),
				TenantID:        tenantID,
				ArtifactTypeID:  typeID,
				ArtifactTypeKey: TypeKeyTextDraft,
				PlanExecutionID: uuid.NullUUID{UUID: wantedExecutionID, Valid: true},
				UpdatedAt:       time.Now().UTC(),
			},
			uuid.New(): {
				ID:              uuid.New(),
				TenantID:        tenantID,
				ArtifactTypeID:  typeID,
				ArtifactTypeKey: TypeKeyTextDraft,
				PlanExecutionID: uuid.NullUUID{UUID: otherExecutionID, Valid: true},
				UpdatedAt:       time.Now().Add(-time.Minute).UTC(),
			},
		},
	}

	got, err := repo.ListArtifacts(context.Background(), tenantID, ArtifactListFilter{
		PlanExecutionID: &wantedExecutionID,
		Limit:           10,
	})
	if err != nil {
		t.Fatalf("ListArtifacts: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("artifacts = %d, want 1", len(got))
	}
	if !got[0].PlanExecutionID.Valid || got[0].PlanExecutionID.UUID != wantedExecutionID {
		t.Fatalf("plan_execution_id = %v, want %s", got[0].PlanExecutionID, wantedExecutionID)
	}
}

func ptrString(value string) *string {
	return &value
}
