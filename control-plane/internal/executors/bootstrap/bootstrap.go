package bootstrap

import (
	"github.com/harpia/control-plane/internal/artifacts"
	"github.com/harpia/control-plane/internal/executors/integrations/image"
	"github.com/harpia/control-plane/internal/executors/integrations/linkedin"
	"github.com/harpia/control-plane/internal/executors/integrations/rss"
	"github.com/harpia/control-plane/internal/executors/runtime"
)

// Dependencies wires infrastructure ports required by the executor runtime.
type Dependencies struct {
	ArtifactRepo            runtime.ArtifactRepository
	PayloadStore            artifacts.PayloadStore
	FeedFetcher             rss.FeedFetcher
	LinkedInPublisher       linkedin.LinkedInPublisher
	ImageProviderAPIKeyEnvs map[string]string
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
	if deps.LinkedInPublisher == nil {
		deps.LinkedInPublisher = linkedin.NewNoopPublisher()
	}

	imageResolver := image.NewProviderResolver(deps.ImageProviderAPIKeyEnvs)

	configValidators := runtime.NewConfigValidatorRegistry(
		rss.NewConfigValidator(),
		linkedin.NewConfigValidator(),
		image.NewConfigValidator(),
	)

	return &Runtime{
		Integrations: runtime.NewIntegrationRegistry(
			rss.NewHandler(artifactStore, deps.FeedFetcher),
			linkedin.NewHandler(artifactStore, deps.LinkedInPublisher),
			image.NewHandler(artifactStore, imageResolver),
		),
		ArtifactStore:    artifactStore,
		ConfigValidators: configValidators,
	}
}

// DefaultConfigValidators returns the SKU config validators used by API and plan readiness checks.
func DefaultConfigValidators() *runtime.ConfigValidatorRegistry {
	return runtime.NewConfigValidatorRegistry(
		rss.NewConfigValidator(),
		linkedin.NewConfigValidator(),
		image.NewConfigValidator(),
	)
}
