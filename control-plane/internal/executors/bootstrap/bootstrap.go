package bootstrap

import (
	"github.com/harpia/control-plane/internal/artifacts"
	"github.com/harpia/control-plane/internal/executors/integrations/rss"
	"github.com/harpia/control-plane/internal/executors/runtime"
)

// Dependencies wires infrastructure ports required by the executor runtime.
type Dependencies struct {
	ArtifactRepo runtime.ArtifactRepository
	PayloadStore artifacts.PayloadStore
	FeedFetcher  rss.FeedFetcher
}

// Runtime owns integration execution wiring for the control-plane worker.
type Runtime struct {
	Integrations     runtime.IntegrationRunner
	ArtifactStore    runtime.ExecutorArtifactStore
	ConfigValidators *runtime.ConfigValidatorRegistry
}

// NewRuntime constructs the integration registry, artifact store, and SKU validators.
func NewRuntime(deps Dependencies) *Runtime {
	artifactStore := runtime.NewExecutorArtifactStore(deps.ArtifactRepo, deps.PayloadStore)
	if deps.FeedFetcher == nil {
		deps.FeedFetcher = rss.NewHTTPFeedFetcher(nil)
	}

	configValidators := runtime.NewConfigValidatorRegistry(
		rss.NewConfigValidator(),
	)

	return &Runtime{
		Integrations: runtime.NewIntegrationRegistry(
			rss.NewHandler(artifactStore, deps.FeedFetcher),
		),
		ArtifactStore:    artifactStore,
		ConfigValidators: configValidators,
	}
}

// DefaultConfigValidators returns the SKU config validators used by API and plan readiness checks.
func DefaultConfigValidators() *runtime.ConfigValidatorRegistry {
	return runtime.NewConfigValidatorRegistry(
		rss.NewConfigValidator(),
	)
}
