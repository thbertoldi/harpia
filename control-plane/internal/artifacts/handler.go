package artifacts

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	artifactsv1 "github.com/harpia/control-plane/gen/harpia/artifacts/v1"
	"github.com/harpia/control-plane/internal/identity"
)

type Handler struct {
	repo  RepositoryAPI
	store PayloadStore
}

type RepositoryAPI interface {
	RegisterType(ctx context.Context, artifactType *ArtifactType) (*ArtifactType, error)
	GetTypeByID(ctx context.Context, typeID uuid.UUID) (*ArtifactType, error)
	GetTypeByKey(ctx context.Context, key string) (*ArtifactType, error)
	CreateArtifact(ctx context.Context, artifact *Artifact) (*Artifact, error)
	GetArtifact(ctx context.Context, tenantID, artifactID uuid.UUID) (*Artifact, error)
}

func NewHandler(repo RepositoryAPI, store PayloadStore) (*Handler, error) {
	if repo == nil {
		return nil, errors.New("artifacts: repository is required")
	}
	if store == nil {
		return nil, errors.New("artifacts: payload store is required")
	}
	return &Handler{repo: repo, store: store}, nil
}

func (h *Handler) RegisterArtifactType(ctx context.Context, req *connect.Request[artifactsv1.RegisterArtifactTypeRequest]) (*connect.Response[artifactsv1.RegisterArtifactTypeResponse], error) {
	if req.Msg.Key == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("key is required"))
	}
	if req.Msg.SchemaRef == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("schema_ref is required"))
	}

	version := req.Msg.Version
	if version <= 0 {
		version = 1
	}

	created, err := h.repo.RegisterType(ctx, &ArtifactType{
		Key:         req.Msg.Key,
		SchemaRef:   req.Msg.SchemaRef,
		Version:     version,
		Description: req.Msg.Description,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&artifactsv1.RegisterArtifactTypeResponse{
		ArtifactType: typeToProto(created),
	}), nil
}

func (h *Handler) GetArtifactType(ctx context.Context, req *connect.Request[artifactsv1.GetArtifactTypeRequest]) (*connect.Response[artifactsv1.GetArtifactTypeResponse], error) {
	var (
		artifactType *ArtifactType
		err          error
	)

	switch {
	case req.Msg.ArtifactTypeId != "":
		typeID, parseErr := uuid.Parse(req.Msg.ArtifactTypeId)
		if parseErr != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, parseErr)
		}
		artifactType, err = h.repo.GetTypeByID(ctx, typeID)
	case req.Msg.Key != "":
		artifactType, err = h.repo.GetTypeByKey(ctx, req.Msg.Key)
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("artifact_type_id or key is required"))
	}

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&artifactsv1.GetArtifactTypeResponse{
		ArtifactType: typeToProto(artifactType),
	}), nil
}

func (h *Handler) CreateArtifact(ctx context.Context, req *connect.Request[artifactsv1.CreateArtifactRequest]) (*connect.Response[artifactsv1.CreateArtifactResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}
	if req.Msg.StorageUri == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("storage_uri is required"))
	}
	if req.Msg.ContentHash == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("content_hash is required"))
	}

	typeID, err := uuid.Parse(req.Msg.ArtifactTypeId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if _, err := h.repo.GetTypeByID(ctx, typeID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("artifact type not found: %w", err))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	created, err := h.repo.CreateArtifact(ctx, &Artifact{
		ID:             uuid.New(),
		TenantID:       tenantID,
		ArtifactTypeID: typeID,
		StorageURI:     req.Msg.StorageUri,
		ContentHash:    req.Msg.ContentHash,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&artifactsv1.CreateArtifactResponse{
		Artifact: artifactToProto(created),
	}), nil
}

func (h *Handler) CreateArtifactWithPayload(ctx context.Context, req *connect.Request[artifactsv1.CreateArtifactWithPayloadRequest]) (*connect.Response[artifactsv1.CreateArtifactWithPayloadResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}

	artifactType, err := h.resolveArtifactType(ctx, req.Msg.ArtifactTypeId, req.Msg.ArtifactTypeKey)
	if err != nil {
		return nil, err
	}

	if err := ValidatePayload(artifactType.Key, req.Msg.PayloadJson); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	artifactID := uuid.New()
	stepExecutionID := ""
	if req.Msg.StepExecutionId != nil {
		stepExecutionID = *req.Msg.StepExecutionId
	}

	objectPath := ArtifactObjectPath(artifactID, stepExecutionID)
	storageURI, err := h.store.Put(ctx, objectPath, req.Msg.PayloadJson)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	created, err := h.repo.CreateArtifact(ctx, &Artifact{
		ID:             artifactID,
		TenantID:       tenantID,
		ArtifactTypeID: artifactType.ID,
		StorageURI:     storageURI,
		ContentHash:    ContentHash(req.Msg.PayloadJson),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&artifactsv1.CreateArtifactWithPayloadResponse{
		Artifact: artifactToProto(created),
	}), nil
}

func (h *Handler) GetArtifact(ctx context.Context, req *connect.Request[artifactsv1.GetArtifactRequest]) (*connect.Response[artifactsv1.GetArtifactResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}

	artifactID, err := uuid.Parse(req.Msg.ArtifactId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	artifact, err := h.repo.GetArtifact(ctx, tenantID, artifactID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&artifactsv1.GetArtifactResponse{
		Artifact: artifactToProto(artifact),
	}), nil
}

func (h *Handler) GetArtifactPayload(ctx context.Context, req *connect.Request[artifactsv1.GetArtifactPayloadRequest]) (*connect.Response[artifactsv1.GetArtifactPayloadResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}

	artifactID, err := uuid.Parse(req.Msg.ArtifactId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	artifact, err := h.repo.GetArtifact(ctx, tenantID, artifactID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	payload, err := h.store.Get(ctx, artifact.StorageURI)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&artifactsv1.GetArtifactPayloadResponse{
		PayloadJson: payload,
		ContentHash: artifact.ContentHash,
	}), nil
}

func (h *Handler) PreviewArtifact(ctx context.Context, req *connect.Request[artifactsv1.PreviewArtifactRequest]) (*connect.Response[artifactsv1.PreviewArtifactResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}

	artifactID, err := uuid.Parse(req.Msg.ArtifactId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	artifact, err := h.repo.GetArtifact(ctx, tenantID, artifactID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	artifactType, err := h.repo.GetTypeByID(ctx, artifact.ArtifactTypeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("artifact type not found: %w", err))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	payload, err := h.store.Get(ctx, artifact.StorageURI)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	preview, err := BuildPreview(artifactType.Key, payload)
	if err != nil {
		if errors.Is(err, ErrInvalidPayload) {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(preview), nil
}

func (h *Handler) resolveArtifactType(ctx context.Context, typeID, typeKey string) (*ArtifactType, error) {
	switch {
	case typeID != "":
		parsed, err := uuid.Parse(typeID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		artifactType, err := h.repo.GetTypeByID(ctx, parsed)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("artifact type not found: %w", err))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		return artifactType, nil
	case typeKey != "":
		artifactType, err := h.repo.GetTypeByKey(ctx, typeKey)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("artifact type not found: %w", err))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		return artifactType, nil
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("artifact_type_id or artifact_type_key is required"))
	}
}

func typeToProto(t *ArtifactType) *artifactsv1.ArtifactType {
	return &artifactsv1.ArtifactType{
		Id:          t.ID.String(),
		Key:         t.Key,
		SchemaRef:   t.SchemaRef,
		Version:     t.Version,
		Description: t.Description,
	}
}

func artifactToProto(a *Artifact) *artifactsv1.Artifact {
	return &artifactsv1.Artifact{
		Id:             a.ID.String(),
		ArtifactTypeId: a.ArtifactTypeID.String(),
		TenantId:       a.TenantID.String(),
		StorageUri:     a.StorageURI,
		ContentHash:    a.ContentHash,
		CreatedAt:      a.CreatedAt.Format(time.RFC3339),
	}
}
