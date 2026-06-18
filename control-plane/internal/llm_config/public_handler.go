package llm_config

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	llmconfigv1 "github.com/harpia/control-plane/gen/harpia/llm_config/v1"
)

// PublicHandler exposes tenant-facing LLM config RPCs on the public Connect
// mux. The internal resolver RPC is intentionally unavailable here.
type PublicHandler struct {
	*Handler
}

// NewPublicHandler wraps a handler for public registration.
func NewPublicHandler(h *Handler) *PublicHandler {
	return &PublicHandler{Handler: h}
}

// ResolveLLMProviderForTenant is internal-only and must not be reachable from
// the public API surface.
func (PublicHandler) ResolveLLMProviderForTenant(
	context.Context,
	*connect.Request[llmconfigv1.ResolveLLMProviderForTenantRequest],
) (*connect.Response[llmconfigv1.ResolveLLMProviderForTenantResponse], error) {
	return nil, connect.NewError(
		connect.CodePermissionDenied,
		errors.New("ResolveLLMProviderForTenant is internal-only"),
	)
}
