package executors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	executorsv1 "github.com/harpia/control-plane/gen/harpia/executors/v1"
	"github.com/harpia/control-plane/internal/identity"
)

type Handler struct {
	repo             *Repository
	configValidators *ConfigValidatorRegistry
}

func NewHandler(repo *Repository, configValidators *ConfigValidatorRegistry) (*Handler, error) {
	if repo == nil {
		return nil, errors.New("executors: repository is required")
	}
	if configValidators == nil {
		configValidators = DefaultConfigValidators()
	}
	return &Handler{repo: repo, configValidators: configValidators}, nil
}

func (h *Handler) ListExecutorSKUs(ctx context.Context, req *connect.Request[executorsv1.ListExecutorSKUsRequest], stream *connect.ServerStream[executorsv1.ListExecutorSKUsResponse]) error {
	if _, err := identity.RequireSelectedTenant(ctx); err != nil {
		return err
	}

	kindFilter, err := protoKindToDB(req.Msg.KindFilter)
	if err != nil {
		return connect.NewError(connect.CodeInvalidArgument, err)
	}

	limit, offset, err := pageParams(req.Msg.PageSize, req.Msg.PageToken)
	if err != nil {
		return connect.NewError(connect.CodeInvalidArgument, err)
	}

	skus, err := h.repo.ListSKUs(ctx, kindFilter, limit+1, offset)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	hasNext := len(skus) > limit
	if hasNext {
		skus = skus[:limit]
	}

	response := &executorsv1.ListExecutorSKUsResponse{
		ExecutorSkus: make([]*executorsv1.ExecutorSKU, 0, len(skus)),
	}
	for i := range skus {
		response.ExecutorSkus = append(response.ExecutorSkus, skuToProto(&skus[i]))
	}
	if hasNext {
		response.NextPageToken = strconv.Itoa(offset + limit)
	}
	return stream.Send(response)
}

func (h *Handler) GetExecutorSKU(ctx context.Context, req *connect.Request[executorsv1.GetExecutorSKURequest]) (*connect.Response[executorsv1.GetExecutorSKUResponse], error) {
	if _, err := identity.RequireSelectedTenant(ctx); err != nil {
		return nil, err
	}

	var sku *ExecutorSKU
	var err error
	switch selector := req.Msg.Selector.(type) {
	case *executorsv1.GetExecutorSKURequest_ExecutorSkuId:
		skuID, parseErr := uuid.Parse(selector.ExecutorSkuId)
		if parseErr != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, parseErr)
		}
		sku, err = h.repo.GetSKUByID(ctx, skuID)
	case *executorsv1.GetExecutorSKURequest_Key:
		if selector.Key == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("missing sku key"))
		}
		sku, err = h.repo.GetSKUByKey(ctx, selector.Key)
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("missing sku selector"))
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&executorsv1.GetExecutorSKUResponse{
		ExecutorSku: skuToProto(sku),
	}), nil
}

func (h *Handler) ListExecutorEntitlements(ctx context.Context, req *connect.Request[executorsv1.ListExecutorEntitlementsRequest], stream *connect.ServerStream[executorsv1.ListExecutorEntitlementsResponse]) error {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return err
	}

	var skuID *uuid.UUID
	if req.Msg.ExecutorSkuId != nil && *req.Msg.ExecutorSkuId != "" {
		parsed, parseErr := uuid.Parse(*req.Msg.ExecutorSkuId)
		if parseErr != nil {
			return connect.NewError(connect.CodeInvalidArgument, parseErr)
		}
		skuID = &parsed
	}

	limit, offset, err := pageParams(req.Msg.PageSize, req.Msg.PageToken)
	if err != nil {
		return connect.NewError(connect.CodeInvalidArgument, err)
	}

	entitlements, err := h.repo.ListEntitlements(ctx, tenantID, skuID, limit+1, offset)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	hasNext := len(entitlements) > limit
	if hasNext {
		entitlements = entitlements[:limit]
	}

	response := &executorsv1.ListExecutorEntitlementsResponse{
		Entitlements: make([]*executorsv1.ExecutorEntitlement, 0, len(entitlements)),
	}
	for i := range entitlements {
		response.Entitlements = append(response.Entitlements, entitlementToProto(&entitlements[i]))
	}
	if hasNext {
		response.NextPageToken = strconv.Itoa(offset + limit)
	}
	return stream.Send(response)
}

func (h *Handler) GetExecutorEntitlement(ctx context.Context, req *connect.Request[executorsv1.GetExecutorEntitlementRequest]) (*connect.Response[executorsv1.GetExecutorEntitlementResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}

	entitlementID, err := uuid.Parse(req.Msg.EntitlementId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	entitlement, err := h.repo.GetEntitlementByID(ctx, tenantID, entitlementID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&executorsv1.GetExecutorEntitlementResponse{
		Entitlement: entitlementToProto(entitlement),
	}), nil
}

func (h *Handler) CreateExecutorEntitlement(ctx context.Context, req *connect.Request[executorsv1.CreateExecutorEntitlementRequest]) (*connect.Response[executorsv1.CreateExecutorEntitlementResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}

	skuID, err := uuid.Parse(req.Msg.ExecutorSkuId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if _, err := h.repo.GetSKUByID(ctx, skuID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("executor sku not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	entitlement, err := h.repo.CreateEntitlement(ctx, tenantID, skuID, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&executorsv1.CreateExecutorEntitlementResponse{
		Entitlement: entitlementToProto(entitlement),
	}), nil
}

func (h *Handler) ListExecutorInstallations(ctx context.Context, req *connect.Request[executorsv1.ListExecutorInstallationsRequest], stream *connect.ServerStream[executorsv1.ListExecutorInstallationsResponse]) error {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return err
	}

	kindFilter, err := protoKindToDB(req.Msg.KindFilter)
	if err != nil {
		return connect.NewError(connect.CodeInvalidArgument, err)
	}

	var skuID *uuid.UUID
	if req.Msg.ExecutorSkuId != nil && *req.Msg.ExecutorSkuId != "" {
		parsed, parseErr := uuid.Parse(*req.Msg.ExecutorSkuId)
		if parseErr != nil {
			return connect.NewError(connect.CodeInvalidArgument, parseErr)
		}
		skuID = &parsed
	}

	limit, offset, err := pageParams(req.Msg.PageSize, req.Msg.PageToken)
	if err != nil {
		return connect.NewError(connect.CodeInvalidArgument, err)
	}

	installations, err := h.repo.ListInstallations(ctx, tenantID, kindFilter, skuID, limit+1, offset)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	hasNext := len(installations) > limit
	if hasNext {
		installations = installations[:limit]
	}

	response := &executorsv1.ListExecutorInstallationsResponse{
		Installations: make([]*executorsv1.ExecutorInstallation, 0, len(installations)),
	}
	for i := range installations {
		response.Installations = append(response.Installations, installationToProto(&installations[i]))
	}
	if hasNext {
		response.NextPageToken = strconv.Itoa(offset + limit)
	}
	return stream.Send(response)
}

func (h *Handler) GetExecutorInstallation(ctx context.Context, req *connect.Request[executorsv1.GetExecutorInstallationRequest]) (*connect.Response[executorsv1.GetExecutorInstallationResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}

	installationID, err := uuid.Parse(req.Msg.InstallationId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	installation, err := h.repo.GetInstallationByID(ctx, tenantID, installationID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&executorsv1.GetExecutorInstallationResponse{
		Installation: installationToProto(installation),
	}), nil
}

func (h *Handler) CreateExecutorInstallation(ctx context.Context, req *connect.Request[executorsv1.CreateExecutorInstallationRequest]) (*connect.Response[executorsv1.CreateExecutorInstallationResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}

	skuID, err := uuid.Parse(req.Msg.ExecutorSkuId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	sku, err := h.repo.GetSKUByID(ctx, skuID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("executor sku not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	entitlements, err := h.repo.ListEntitlements(ctx, tenantID, &skuID, 1, 0)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if len(entitlements) == 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("tenant is not entitled to this executor sku"))
	}

	installation := &ExecutorInstallation{
		TenantID:      tenantID,
		ExecutorSKUID: skuID,
		Kind:          sku.Kind,
		DisplayName:   req.Msg.DisplayName,
		Enabled:       true,
		ConfigJSON:    json.RawMessage("{}"),
	}
	if req.Msg.Enabled != nil {
		installation.Enabled = *req.Msg.Enabled
	}
	if installation.DisplayName == "" {
		installation.DisplayName = sku.DisplayName
	}

	switch sku.Kind {
	case KindIntegration:
		status := "disconnected"
		configJSON := json.RawMessage("{}")
		switch detail := req.Msg.InitialDetail.(type) {
		case nil:
		case *executorsv1.CreateExecutorInstallationRequest_Integration:
			if detail.Integration == nil {
				break
			}
			status, err = connectionStatusToDB(detail.Integration.ConnectionStatus)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
			if detail.Integration.ConfigJson != "" {
				if !json.Valid([]byte(detail.Integration.ConfigJson)) {
					return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("integration config_json must be valid JSON"))
				}
				configJSON = json.RawMessage(detail.Integration.ConfigJson)
			}
		default:
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("integration executor installation requires integration detail"))
		}
		if trimmed := strings.TrimSpace(string(configJSON)); trimmed != "" && trimmed != "{}" && trimmed != "null" {
			if err := h.configValidators.Validate(sku.Key, configJSON); err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
		}
		installation.ConnectionStatus = &status
		installation.ConfigJSON = configJSON
	case KindAgent:
		manifestID := sku.Compatibility.ManifestID
		manifestVersion := sku.Compatibility.ManifestVersion
		switch detail := req.Msg.InitialDetail.(type) {
		case nil:
		case *executorsv1.CreateExecutorInstallationRequest_Agent:
			if detail.Agent == nil {
				break
			}
			if detail.Agent.ManifestId != "" {
				manifestID = detail.Agent.ManifestId
			}
			if detail.Agent.ManifestVersion != "" {
				manifestVersion = detail.Agent.ManifestVersion
			}
		default:
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("agent executor installation requires agent detail"))
		}
		if manifestID == "" || manifestVersion == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("agent installation requires manifest_id and manifest_version"))
		}
		installation.ManifestID = &manifestID
		installation.ManifestVersion = &manifestVersion
	default:
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("unsupported executor kind %q", sku.Kind))
	}

	created, err := h.repo.CreateInstallation(ctx, installation)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&executorsv1.CreateExecutorInstallationResponse{
		Installation: installationToProto(created),
	}), nil
}

func (h *Handler) UpdateExecutorInstallation(ctx context.Context, req *connect.Request[executorsv1.UpdateExecutorInstallationRequest]) (*connect.Response[executorsv1.UpdateExecutorInstallationResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}

	installationID, err := uuid.Parse(req.Msg.InstallationId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	existing, err := h.repo.GetInstallationByID(ctx, tenantID, installationID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	sku, err := h.repo.GetSKUByID(ctx, existing.ExecutorSKUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("executor sku not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	entitlements, err := h.repo.ListEntitlements(ctx, tenantID, &existing.ExecutorSKUID, 1, 0)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if len(entitlements) == 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("tenant is not entitled to this executor sku"))
	}

	updated := *existing
	if strings.TrimSpace(req.Msg.DisplayName) != "" {
		updated.DisplayName = strings.TrimSpace(req.Msg.DisplayName)
	}
	if req.Msg.Enabled != nil {
		updated.Enabled = *req.Msg.Enabled
	}

	switch existing.Kind {
	case KindIntegration:
		switch detail := req.Msg.Detail.(type) {
		case nil:
		case *executorsv1.UpdateExecutorInstallationRequest_Integration:
			if detail.Integration == nil {
				break
			}
			status, err := connectionStatusToDB(detail.Integration.ConnectionStatus)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
			configJSON := json.RawMessage("{}")
			if detail.Integration.ConfigJson != "" {
				if !json.Valid([]byte(detail.Integration.ConfigJson)) {
					return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("integration config_json must be valid JSON"))
				}
				configJSON = json.RawMessage(detail.Integration.ConfigJson)
			}
			if trimmed := strings.TrimSpace(string(configJSON)); trimmed != "" && trimmed != "{}" && trimmed != "null" {
				if err := h.configValidators.Validate(sku.Key, configJSON); err != nil {
					return nil, connect.NewError(connect.CodeInvalidArgument, err)
				}
			}
			updated.ConnectionStatus = &status
			updated.ConfigJSON = configJSON
			updated.ManifestID = nil
			updated.ManifestVersion = nil
		default:
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("integration executor installation requires integration detail"))
		}
	case KindAgent:
		switch detail := req.Msg.Detail.(type) {
		case nil:
		case *executorsv1.UpdateExecutorInstallationRequest_Agent:
			if detail.Agent == nil {
				break
			}
			manifestID := strings.TrimSpace(detail.Agent.ManifestId)
			manifestVersion := strings.TrimSpace(detail.Agent.ManifestVersion)
			if manifestID == "" || manifestVersion == "" {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("agent installation requires manifest_id and manifest_version"))
			}
			updated.ManifestID = &manifestID
			updated.ManifestVersion = &manifestVersion
			updated.ConnectionStatus = nil
			updated.ConfigJSON = json.RawMessage("{}")
		default:
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("agent executor installation requires agent detail"))
		}
	default:
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("unsupported executor kind %q", existing.Kind))
	}

	saved, err := h.repo.UpdateInstallation(ctx, &updated)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&executorsv1.UpdateExecutorInstallationResponse{
		Installation: installationToProto(saved),
	}), nil
}

func pageParams(pageSize int32, pageToken string) (limit int, offset int, err error) {
	limit = int(pageSize)
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if pageToken == "" {
		return limit, 0, nil
	}
	offset, err = strconv.Atoi(pageToken)
	if err != nil || offset < 0 {
		return 0, 0, fmt.Errorf("invalid page token %q", pageToken)
	}
	return limit, offset, nil
}

func protoKindToDB(kindFilter *executorsv1.ExecutorKind) (string, error) {
	if kindFilter == nil || *kindFilter == executorsv1.ExecutorKind_EXECUTOR_KIND_UNSPECIFIED {
		return "", nil
	}
	switch *kindFilter {
	case executorsv1.ExecutorKind_EXECUTOR_KIND_INTEGRATION:
		return KindIntegration, nil
	case executorsv1.ExecutorKind_EXECUTOR_KIND_AGENT:
		return KindAgent, nil
	default:
		return "", fmt.Errorf("unsupported executor kind %s", kindFilter.String())
	}
}

func dbKindToProto(kind string) executorsv1.ExecutorKind {
	switch kind {
	case KindIntegration:
		return executorsv1.ExecutorKind_EXECUTOR_KIND_INTEGRATION
	case KindAgent:
		return executorsv1.ExecutorKind_EXECUTOR_KIND_AGENT
	default:
		return executorsv1.ExecutorKind_EXECUTOR_KIND_UNSPECIFIED
	}
}

func connectionStatusToDB(status executorsv1.ConnectionStatus) (string, error) {
	switch status {
	case executorsv1.ConnectionStatus_CONNECTION_STATUS_UNSPECIFIED, executorsv1.ConnectionStatus_CONNECTION_STATUS_DISCONNECTED:
		return "disconnected", nil
	case executorsv1.ConnectionStatus_CONNECTION_STATUS_CONNECTING:
		return "connecting", nil
	case executorsv1.ConnectionStatus_CONNECTION_STATUS_CONNECTED:
		return "connected", nil
	case executorsv1.ConnectionStatus_CONNECTION_STATUS_ERROR:
		return "error", nil
	default:
		return "", fmt.Errorf("unsupported connection status %s", status.String())
	}
}

func connectionStatusToProto(status string) executorsv1.ConnectionStatus {
	switch status {
	case "disconnected":
		return executorsv1.ConnectionStatus_CONNECTION_STATUS_DISCONNECTED
	case "connecting":
		return executorsv1.ConnectionStatus_CONNECTION_STATUS_CONNECTING
	case "connected":
		return executorsv1.ConnectionStatus_CONNECTION_STATUS_CONNECTED
	case "error":
		return executorsv1.ConnectionStatus_CONNECTION_STATUS_ERROR
	default:
		return executorsv1.ConnectionStatus_CONNECTION_STATUS_UNSPECIFIED
	}
}

func skuToProto(sku *ExecutorSKU) *executorsv1.ExecutorSKU {
	return &executorsv1.ExecutorSKU{
		Id:          sku.ID.String(),
		Key:         sku.Key,
		DisplayName: sku.DisplayName,
		Description: sku.Description,
		Kind:        dbKindToProto(sku.Kind),
		ListPrice: &executorsv1.ListPrice{
			PriceCents: sku.PriceCents,
			Currency:   sku.Currency,
		},
		Compatibility: &executorsv1.CompatibilityMetadata{
			InputArtifactTypeKeys:  sku.Compatibility.InputArtifactTypeKeys,
			OutputArtifactTypeKeys: sku.Compatibility.OutputArtifactTypeKeys,
			ConnectionType:         sku.Compatibility.ConnectionType,
			ManifestId:             sku.Compatibility.ManifestID,
			ManifestVersion:        sku.Compatibility.ManifestVersion,
		},
		CreatedAt: sku.CreatedAt.Format(time.RFC3339),
		UpdatedAt: sku.UpdatedAt.Format(time.RFC3339),
	}
}

func entitlementToProto(entitlement *ExecutorEntitlement) *executorsv1.ExecutorEntitlement {
	proto := &executorsv1.ExecutorEntitlement{
		Id:            entitlement.ID.String(),
		TenantId:      entitlement.TenantID.String(),
		ExecutorSkuId: entitlement.ExecutorSKUID.String(),
		GrantedAt:     entitlement.GrantedAt.Format(time.RFC3339),
	}
	if entitlement.GrantedBy != nil {
		grantedBy := entitlement.GrantedBy.String()
		proto.GrantedBy = &grantedBy
	}
	return proto
}

func installationToProto(installation *ExecutorInstallation) *executorsv1.ExecutorInstallation {
	proto := &executorsv1.ExecutorInstallation{
		Id:            installation.ID.String(),
		TenantId:      installation.TenantID.String(),
		ExecutorSkuId: installation.ExecutorSKUID.String(),
		Kind:          dbKindToProto(installation.Kind),
		DisplayName:   installation.DisplayName,
		Enabled:       installation.Enabled,
		CreatedAt:     installation.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     installation.UpdatedAt.Format(time.RFC3339),
	}

	switch installation.Kind {
	case KindIntegration:
		status := "disconnected"
		if installation.ConnectionStatus != nil {
			status = *installation.ConnectionStatus
		}
		proto.Detail = &executorsv1.ExecutorInstallation_Integration{
			Integration: &executorsv1.IntegrationInstallation{
				ConnectionStatus: connectionStatusToProto(status),
				ConfigJson:       string(installation.ConfigJSON),
			},
		}
	case KindAgent:
		manifestID := ""
		manifestVersion := ""
		if installation.ManifestID != nil {
			manifestID = *installation.ManifestID
		}
		if installation.ManifestVersion != nil {
			manifestVersion = *installation.ManifestVersion
		}
		proto.Detail = &executorsv1.ExecutorInstallation_Agent{
			Agent: &executorsv1.AgentInstallation{
				ManifestId:      manifestID,
				ManifestVersion: manifestVersion,
			},
		}
	}

	return proto
}
