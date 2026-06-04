package agents

import (
	"context"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	agentsv1 "github.com/harpia/control-plane/gen/harpia/agents/v1"
)

type AgentHandler struct {
	repo *Repository
}

func NewAgentHandler(repo *Repository) *AgentHandler {
	return &AgentHandler{repo: repo}
}

func (h *AgentHandler) RegisterAgentType(ctx context.Context, req *connect.Request[agentsv1.RegisterAgentTypeRequest]) (*connect.Response[agentsv1.RegisterAgentTypeResponse], error) {
	agentType := &AgentType{
		Name:        req.Msg.Name,
		Description: req.Msg.Description,
		Enabled:     true,
	}

	created, err := h.repo.Create(ctx, agentType)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&agentsv1.RegisterAgentTypeResponse{
		AgentType: &agentsv1.AgentType{
			Id:               created.ID.String(),
			Name:             created.Name,
			Description:      created.Description,
			CapabilitiesText: "",
			CreatedAt:        created.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		},
	}), nil
}

func (h *AgentHandler) ListAgentTypes(ctx context.Context, req *connect.Request[agentsv1.ListAgentTypesRequest], stream *connect.ServerStream[agentsv1.ListAgentTypesResponse]) error {
	return nil
}

func (h *AgentHandler) MatchAgent(ctx context.Context, req *connect.Request[agentsv1.MatchAgentRequest]) (*connect.Response[agentsv1.MatchAgentResponse], error) {
	return connect.NewResponse(&agentsv1.MatchAgentResponse{
		Matches: []*agentsv1.AgentMatch{},
	}), nil
}

func (h *AgentHandler) ExecuteTask(ctx context.Context, req *connect.Request[agentsv1.ExecuteTaskRequest], stream *connect.ServerStream[agentsv1.ExecuteTaskResponse]) error {
	return nil
}

func (h *AgentHandler) ContinueExecution(ctx context.Context, req *connect.Request[agentsv1.ContinueExecutionRequest]) (*connect.Response[agentsv1.ContinueExecutionResponse], error) {
	return connect.NewResponse(&agentsv1.ContinueExecutionResponse{
		AgentInstanceId: uuid.New().String(),
		Status:          agentsv1.AgentInstanceStatus_AGENT_INSTANCE_STATUS_EXECUTING,
		Message:         "execution resumed",
	}), nil
}
