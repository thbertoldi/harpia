package artifacts

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
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
	ListArtifacts(ctx context.Context, tenantID uuid.UUID, filter ArtifactListFilter) ([]Artifact, error)
	CreateArtifactVersion(ctx context.Context, artifact *Artifact, version *ArtifactVersion) (*ArtifactVersion, *Artifact, error)
	ListArtifactVersions(ctx context.Context, tenantID, artifactID uuid.UUID, limit, offset int) ([]ArtifactVersion, string, error)
	GetArtifactVersion(ctx context.Context, tenantID, artifactID, versionID uuid.UUID) (*ArtifactVersion, error)
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
	stepExecutionIDRaw := strings.TrimSpace(req.Msg.GetStepExecutionId())
	stepExecutionID, err := optionalUUID(req.Msg.StepExecutionId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("step_execution_id: %w", err))
	}
	planConfigurationID, err := optionalUUID(req.Msg.PlanConfigurationId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("plan_configuration_id: %w", err))
	}
	planExecutionID, err := optionalUUID(req.Msg.PlanExecutionId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("plan_execution_id: %w", err))
	}

	objectPath := ArtifactObjectPath(artifactID, stepExecutionIDRaw)
	storageURI, err := h.store.Put(ctx, objectPath, req.Msg.PayloadJson)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	created, err := h.repo.CreateArtifact(ctx, &Artifact{
		ID:                  artifactID,
		TenantID:            tenantID,
		ArtifactTypeID:      artifactType.ID,
		StorageURI:          storageURI,
		ContentHash:         ContentHash(req.Msg.PayloadJson),
		PlanConfigurationID: planConfigurationID,
		PlanExecutionID:     planExecutionID,
		StepExecutionID:     stepExecutionID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !created.CurrentVersionID.Valid {
		return nil, connect.NewError(connect.CodeInternal, errors.New("created artifact is missing its initial version"))
	}
	createdVersion, err := h.repo.GetArtifactVersion(ctx, tenantID, created.ID, created.CurrentVersionID.UUID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load created artifact version: %w", err))
	}

	return connect.NewResponse(&artifactsv1.CreateArtifactWithPayloadResponse{
		Artifact:        artifactToProto(created),
		ArtifactVersion: artifactVersionToProto(createdVersion),
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

func (h *Handler) ListArtifacts(ctx context.Context, req *connect.Request[artifactsv1.ListArtifactsRequest], stream *connect.ServerStream[artifactsv1.ListArtifactsResponse]) error {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return err
	}
	limit, offset, err := pageParams(req.Msg.PageSize, req.Msg.PageToken)
	if err != nil {
		return connect.NewError(connect.CodeInvalidArgument, err)
	}

	filter := ArtifactListFilter{
		Limit:           limit + 1,
		Offset:          offset,
		ArtifactTypeKey: strings.TrimSpace(req.Msg.GetArtifactTypeKey()),
	}
	if req.Msg.PlanConfigurationId != nil {
		parsed, err := parseOptionalUUIDValue(req.Msg.GetPlanConfigurationId())
		if err != nil {
			return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("plan_configuration_id: %w", err))
		}
		if parsed != nil {
			filter.PlanConfigurationID = parsed
		}
	}
	if req.Msg.PlanExecutionId != nil {
		parsed, err := parseOptionalUUIDValue(req.Msg.GetPlanExecutionId())
		if err != nil {
			return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("plan_execution_id: %w", err))
		}
		if parsed != nil {
			filter.PlanExecutionID = parsed
		}
	}

	items, err := h.repo.ListArtifacts(ctx, tenantID, filter)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	nextPageToken := ""
	if len(items) > limit {
		items = items[:limit]
		nextPageToken = strconv.Itoa(offset + limit)
	}

	resp := &artifactsv1.ListArtifactsResponse{
		Artifacts:     make([]*artifactsv1.Artifact, 0, len(items)),
		NextPageToken: nextPageToken,
	}
	for i := range items {
		resp.Artifacts = append(resp.Artifacts, artifactToProto(&items[i]))
	}
	return stream.Send(resp)
}

func (h *Handler) ListArtifactVersions(ctx context.Context, req *connect.Request[artifactsv1.ListArtifactVersionsRequest]) (*connect.Response[artifactsv1.ListArtifactVersionsResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}
	artifactID, err := uuid.Parse(req.Msg.ArtifactId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	limit, offset, err := pageParams(req.Msg.PageSize, req.Msg.PageToken)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	versions, nextToken, err := h.repo.ListArtifactVersions(ctx, tenantID, artifactID, limit, offset)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &artifactsv1.ListArtifactVersionsResponse{
		Versions:      make([]*artifactsv1.ArtifactVersion, 0, len(versions)),
		NextPageToken: nextToken,
	}
	for i := range versions {
		resp.Versions = append(resp.Versions, artifactVersionToProto(&versions[i]))
	}
	return connect.NewResponse(resp), nil
}

func (h *Handler) GetArtifactVersion(ctx context.Context, req *connect.Request[artifactsv1.GetArtifactVersionRequest]) (*connect.Response[artifactsv1.GetArtifactVersionResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}
	artifactID, err := uuid.Parse(req.Msg.ArtifactId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	versionID, err := uuid.Parse(req.Msg.ArtifactVersionId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	version, err := h.repo.GetArtifactVersion(ctx, tenantID, artifactID, versionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&artifactsv1.GetArtifactVersionResponse{ArtifactVersion: artifactVersionToProto(version)}), nil
}

func (h *Handler) CreateArtifactVersionWithPayload(ctx context.Context, req *connect.Request[artifactsv1.CreateArtifactVersionWithPayloadRequest]) (*connect.Response[artifactsv1.CreateArtifactVersionWithPayloadResponse], error) {
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
	if strings.TrimSpace(req.Msg.ExpectedContentHash) == "" || req.Msg.ExpectedContentHash != artifact.ContentHash {
		return nil, connect.NewError(connect.CodeFailedPrecondition, ErrContentHashConflict)
	}
	artifactType, err := h.repo.GetTypeByID(ctx, artifact.ArtifactTypeID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := ValidatePayload(artifactType.Key, req.Msg.PayloadJson); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	versions, _, err := h.repo.ListArtifactVersions(ctx, tenantID, artifactID, 1, 0)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	nextVersionNumber := int32(1)
	if len(versions) > 0 {
		nextVersionNumber = versions[0].VersionNumber + 1
	}
	storageURI, err := h.store.Put(ctx, ArtifactVersionObjectPath(artifactID, nextVersionNumber), req.Msg.PayloadJson)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	createdByKind := artifactsv1.ArtifactVersionCreatedByKind_ARTIFACT_VERSION_CREATED_BY_KIND_SYSTEM
	if userID := userIDFromContext(ctx); userID.Valid {
		createdByKind = artifactsv1.ArtifactVersionCreatedByKind_ARTIFACT_VERSION_CREATED_BY_KIND_USER
	}
	version := &ArtifactVersion{
		ID:                    uuid.New(),
		ArtifactID:            artifactID,
		TenantID:              tenantID,
		VersionNumber:         nextVersionNumber,
		StorageURI:            storageURI,
		ContentHash:           ContentHash(req.Msg.PayloadJson),
		SourceVersionID:       artifact.CurrentVersionID,
		SourcePlanExecutionID: artifact.PlanExecutionID,
		SourceStepExecutionID: artifact.StepExecutionID,
		CreatedByUserID:       userIDFromContext(ctx),
		CreatedByKind:         createdByKind,
		EditSummary:           strings.TrimSpace(req.Msg.EditSummary),
	}
	createdVersion, updatedArtifact, err := h.repo.CreateArtifactVersion(ctx, artifact, version)
	if err != nil {
		if errors.Is(err, ErrContentHashConflict) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&artifactsv1.CreateArtifactVersionWithPayloadResponse{
		Artifact:        artifactToProto(updatedArtifact),
		ArtifactVersion: artifactVersionToProto(createdVersion),
	}), nil
}

func (h *Handler) SaveTextArtifactVersion(ctx context.Context, req *connect.Request[artifactsv1.SaveTextArtifactVersionRequest]) (*connect.Response[artifactsv1.SaveTextArtifactVersionResponse], error) {
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
	if req.Msg.ExpectedContentHash != artifact.ContentHash {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("artifact content hash changed"))
	}

	artifactType, err := h.repo.GetTypeByID(ctx, artifact.ArtifactTypeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("artifact type not found: %w", err))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	currentPayload, err := h.store.Get(ctx, artifact.StorageURI)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	editedPayload, err := EditableTextPayload(artifactType.Key, currentPayload, req.Msg.Title, req.Msg.Text)
	if err != nil {
		if errors.Is(err, ErrInvalidPayload) {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := ValidatePayload(artifactType.Key, editedPayload); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	nextVersionNumber := int32(1)
	versions, _, err := h.repo.ListArtifactVersions(ctx, tenantID, artifactID, 1, 0)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if len(versions) > 0 {
		nextVersionNumber = versions[0].VersionNumber + 1
	}

	storageURI, err := h.store.Put(ctx, ArtifactVersionObjectPath(artifactID, nextVersionNumber), editedPayload)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	version := &ArtifactVersion{
		ID:              uuid.New(),
		ArtifactID:      artifactID,
		TenantID:        tenantID,
		VersionNumber:   nextVersionNumber,
		StorageURI:      storageURI,
		ContentHash:     ContentHash(editedPayload),
		SourceVersionID: artifact.CurrentVersionID,
		CreatedByUserID: userIDFromContext(ctx),
		CreatedByKind:   artifactsv1.ArtifactVersionCreatedByKind_ARTIFACT_VERSION_CREATED_BY_KIND_USER,
		EditSummary:     strings.TrimSpace(req.Msg.EditSummary),
	}
	if artifact.PlanExecutionID.Valid {
		version.SourcePlanExecutionID = artifact.PlanExecutionID
	}
	if artifact.StepExecutionID.Valid {
		version.SourceStepExecutionID = artifact.StepExecutionID
	}

	createdVersion, updatedArtifact, err := h.repo.CreateArtifactVersion(ctx, artifact, version)
	if err != nil {
		if errors.Is(err, ErrContentHashConflict) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&artifactsv1.SaveTextArtifactVersionResponse{
		Artifact: artifactToProto(updatedArtifact),
		Version:  artifactVersionToProto(createdVersion),
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

	storageURI := artifact.StorageURI
	contentHash := artifact.ContentHash
	if req.Msg.ArtifactVersionId != nil {
		versionID, err := uuid.Parse(req.Msg.GetArtifactVersionId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		version, err := h.repo.GetArtifactVersion(ctx, tenantID, artifactID, versionID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		storageURI = version.StorageURI
		contentHash = version.ContentHash
	}

	payload, err := h.store.Get(ctx, storageURI)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&artifactsv1.GetArtifactPayloadResponse{
		PayloadJson: payload,
		ContentHash: contentHash,
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

	storageURI := artifact.StorageURI
	if req.Msg.ArtifactVersionId != nil {
		versionID, err := uuid.Parse(req.Msg.GetArtifactVersionId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		version, err := h.repo.GetArtifactVersion(ctx, tenantID, artifactID, versionID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		storageURI = version.StorageURI
	}

	payload, err := h.store.Get(ctx, storageURI)
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
	status := a.Status
	if status == artifactsv1.ArtifactStatus_ARTIFACT_STATUS_UNSPECIFIED {
		status = artifactsv1.ArtifactStatus_ARTIFACT_STATUS_GENERATED
	}
	return &artifactsv1.Artifact{
		Id:                  a.ID.String(),
		ArtifactTypeId:      a.ArtifactTypeID.String(),
		TenantId:            a.TenantID.String(),
		StorageUri:          a.StorageURI,
		ContentHash:         a.ContentHash,
		CreatedAt:           formatTime(a.CreatedAt),
		UpdatedAt:           formatTime(a.UpdatedAt),
		CurrentVersionId:    formatNullUUID(a.CurrentVersionID),
		ArtifactTypeKey:     a.ArtifactTypeKey,
		PlanConfigurationId: formatNullUUID(a.PlanConfigurationID),
		PlanExecutionId:     formatNullUUID(a.PlanExecutionID),
		StepExecutionId:     formatNullUUID(a.StepExecutionID),
		Status:              status,
	}
}

func artifactVersionToProto(v *ArtifactVersion) *artifactsv1.ArtifactVersion {
	return &artifactsv1.ArtifactVersion{
		Id:                    v.ID.String(),
		ArtifactId:            v.ArtifactID.String(),
		VersionNumber:         v.VersionNumber,
		StorageUri:            v.StorageURI,
		ContentHash:           v.ContentHash,
		SourcePlanExecutionId: formatNullUUID(v.SourcePlanExecutionID),
		SourceStepExecutionId: formatNullUUID(v.SourceStepExecutionID),
		SourceVersionId:       formatNullUUID(v.SourceVersionID),
		CreatedByUserId:       formatNullUUID(v.CreatedByUserID),
		CreatedByKind:         v.CreatedByKind,
		EditSummary:           v.EditSummary,
		CreatedAt:             formatTime(v.CreatedAt),
	}
}

func pageParams(pageSize int32, pageToken string) (int, int, error) {
	limit := int(pageSize)
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	if strings.TrimSpace(pageToken) == "" {
		return limit, 0, nil
	}
	offset, err := strconv.Atoi(pageToken)
	if err != nil || offset < 0 {
		return 0, 0, fmt.Errorf("invalid page token %q", pageToken)
	}
	return limit, offset, nil
}

func optionalUUID(raw *string) (uuid.NullUUID, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return uuid.NullUUID{}, nil
	}
	parsed, err := uuid.Parse(strings.TrimSpace(*raw))
	if err != nil {
		return uuid.NullUUID{}, err
	}
	return uuid.NullUUID{UUID: parsed, Valid: true}, nil
}

func parseOptionalUUIDValue(raw string) (*uuid.UUID, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := uuid.Parse(trimmed)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func userIDFromContext(ctx context.Context) uuid.NullUUID {
	rc, ok := identity.RequestContextFrom(ctx)
	if !ok {
		return uuid.NullUUID{}
	}
	parsed, err := uuid.Parse(strings.TrimSpace(rc.UserID))
	if err != nil {
		return uuid.NullUUID{}
	}
	return uuid.NullUUID{UUID: parsed, Valid: true}
}

func formatNullUUID(value uuid.NullUUID) string {
	if !value.Valid {
		return ""
	}
	return value.UUID.String()
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
