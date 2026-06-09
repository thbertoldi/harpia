package agents

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	agentsv1 "github.com/harpia/control-plane/gen/harpia/agents/v1"
	"github.com/harpia/control-plane/internal/cache"
	"github.com/harpia/control-plane/internal/identity"
)

type AgentHandler struct {
	repo       *Repository
	embedder   Embedder
	agentCache *cache.AgentCapabilityCache
}

func NewAgentHandler(repo *Repository, embedder Embedder, agentCache *cache.AgentCapabilityCache) (*AgentHandler, error) {
	if repo == nil {
		return nil, errors.New("agents: repository is required")
	}
	if embedder == nil {
		return nil, errors.New("agents: embedder is required")
	}
	return &AgentHandler{repo: repo, embedder: embedder, agentCache: agentCache}, nil
}

func (h *AgentHandler) RegisterAgentType(ctx context.Context, req *connect.Request[agentsv1.RegisterAgentTypeRequest]) (*connect.Response[agentsv1.RegisterAgentTypeResponse], error) {
	if _, err := identity.RequireSelectedTenant(ctx); err != nil {
		return nil, err
	}

	agentType := &AgentType{
		Name:        req.Msg.Name,
		Description: req.Msg.Description,
		Enabled:     true,
	}

	created, err := h.repo.Create(ctx, agentType)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	slog.Info("agent type registered — consider invalidating related capability cache entries", "agent_id", created.ID.String())

	return connect.NewResponse(&agentsv1.RegisterAgentTypeResponse{
		AgentType: &agentsv1.AgentType{
			Id:               created.ID.String(),
			Name:             created.Name,
			Description:      created.Description,
			CapabilitiesText: "",
			CreatedAt:        created.CreatedAt.Format(time.RFC3339),
		},
	}), nil
}

func (h *AgentHandler) ListAgentTypes(ctx context.Context, req *connect.Request[agentsv1.ListAgentTypesRequest], stream *connect.ServerStream[agentsv1.ListAgentTypesResponse]) error {
	if _, err := identity.RequireSelectedTenant(ctx); err != nil {
		return err
	}

	agentTypes, err := h.repo.List(ctx, true)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	response := &agentsv1.ListAgentTypesResponse{
		AgentTypes: make([]*agentsv1.AgentType, 0, len(agentTypes)),
	}
	for i := range agentTypes {
		response.AgentTypes = append(response.AgentTypes, agentTypeToProto(&agentTypes[i]))
	}
	return stream.Send(response)
}

func (h *AgentHandler) MatchAgent(ctx context.Context, req *connect.Request[agentsv1.MatchAgentRequest]) (*connect.Response[agentsv1.MatchAgentResponse], error) {
	if _, err := identity.RequireTenant(ctx, req.Msg.TenantId); err != nil {
		return nil, err
	}

	if h.agentCache != nil {
		cachedAgentID, err := h.agentCache.GetBestAgent(ctx, req.Msg.TaskDescription)
		if err == nil && cachedAgentID != "" {
			return connect.NewResponse(&agentsv1.MatchAgentResponse{
				Matches: []*agentsv1.AgentMatch{
					{
						AgentType: &agentsv1.AgentType{
							Id: cachedAgentID,
						},
						SimilarityScore: 1.0,
					},
				},
			}), nil
		}
	}

	embedding, err := h.embedder.Embed(ctx, req.Msg.TaskDescription)
	if err != nil {
		slog.Warn("failed to generate embedding, falling back to empty results", "error", err)
		return connect.NewResponse(&agentsv1.MatchAgentResponse{
			Matches: []*agentsv1.AgentMatch{},
		}), nil
	}

	limit := int(req.Msg.MaxResults)
	if limit <= 0 {
		limit = 10
	}

	results, err := h.repo.MatchByCapability(ctx, embedding, limit)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	matches := make([]*agentsv1.AgentMatch, 0, len(results))
	for _, at := range results {
		matches = append(matches, &agentsv1.AgentMatch{
			AgentType: &agentsv1.AgentType{
				Id:          at.ID.String(),
				Name:        at.Name,
				Description: at.Description,
				CreatedAt:   at.CreatedAt.Format(time.RFC3339),
			},
			SimilarityScore: at.Similarity,
		})
	}

	if h.agentCache != nil && len(results) > 0 {
		if err := h.agentCache.SetBestAgent(ctx, req.Msg.TaskDescription, results[0].ID.String()); err != nil {
			slog.Warn("failed to cache best agent match", "error", err)
		}
	}

	return connect.NewResponse(&agentsv1.MatchAgentResponse{
		Matches: matches,
	}), nil
}

func agentTypeToProto(agentType *AgentType) *agentsv1.AgentType {
	return &agentsv1.AgentType{
		Id:          agentType.ID.String(),
		Name:        agentType.Name,
		Description: agentType.Description,
		CreatedAt:   agentType.CreatedAt.Format(time.RFC3339),
	}
}

func (h *AgentHandler) ExecuteTask(ctx context.Context, req *connect.Request[agentsv1.ExecuteTaskRequest], stream *connect.ServerStream[agentsv1.ExecuteTaskResponse]) error {
	_, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	return err
}

func (h *AgentHandler) ContinueExecution(ctx context.Context, req *connect.Request[agentsv1.ContinueExecutionRequest]) (*connect.Response[agentsv1.ContinueExecutionResponse], error) {
	if _, err := identity.RequireTenant(ctx, req.Msg.TenantId); err != nil {
		return nil, err
	}

	return connect.NewResponse(&agentsv1.ContinueExecutionResponse{
		AgentInstanceId: uuid.New().String(),
		Status:          agentsv1.AgentInstanceStatus_AGENT_INSTANCE_STATUS_EXECUTING,
		Message:         "execution resumed",
	}), nil
}
