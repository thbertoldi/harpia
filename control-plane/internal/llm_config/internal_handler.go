package llm_config

import (
	"net/http"

	"connectrpc.com/connect"

	llmconfigv1 "github.com/harpia/control-plane/gen/harpia/llm_config/v1"
	llmconfigv1connect "github.com/harpia/control-plane/gen/harpia/llm_config/v1/llm_configv1connect"
)

const internalLLMConfigPrefix = "/internal"

// NewInternalResolveHandler mounts ResolveLLMProviderForTenant on the trusted
// internal transport only. Agent-runtime should call this prefix with
// HARPIA_INTERNAL_AUTH_TOKEN, not the public user-facing Connect mux.
func NewInternalResolveHandler(h *Handler, opts ...connect.HandlerOption) (string, http.Handler) {
	methods := llmconfigv1.File_harpia_llm_config_v1_llm_config_proto.Services().ByName("LLMConfigService").Methods()
	procedure := internalLLMConfigPrefix + llmconfigv1connect.LLMConfigServiceResolveLLMProviderForTenantProcedure
	resolveHandler := connect.NewUnaryHandler(
		procedure,
		h.ResolveLLMProviderForTenant,
		connect.WithSchema(methods.ByName("ResolveLLMProviderForTenant")),
		connect.WithHandlerOptions(opts...),
	)
	mountPath := internalLLMConfigPrefix + "/harpia.llm_config.v1.LLMConfigService/"
	return mountPath, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == procedure {
			resolveHandler.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})
}
