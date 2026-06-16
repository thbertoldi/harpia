package executors

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CatalogSeed struct {
	Key           string
	DisplayName   string
	Description   string
	Kind          string
	PriceCents    int64
	Currency      string
	Compatibility CompatibilityMetadata
}

var DefaultCatalogSeeds = []CatalogSeed{
	{
		Key:         SKURSSNewsFeed,
		DisplayName: "RSS News Feed",
		Description: "Fetches curated news articles from configured RSS feeds.",
		Kind:        KindIntegration,
		PriceCents:  500,
		Currency:    "USD",
		Compatibility: CompatibilityMetadata{
			InputArtifactTypeKeys:  []string{"harpia.artifacts.v1.DateRange"},
			OutputArtifactTypeKeys: []string{"harpia.artifacts.v1.NewsList"},
			ConnectionType:         "rss_feed",
		},
	},
	{
		Key:         SKULinkedInPublish,
		DisplayName: "LinkedIn Publish",
		Description: "Publishes a LinkedIn post draft through the tenant OAuth connection.",
		Kind:        KindIntegration,
		PriceCents:  1000,
		Currency:    "USD",
		Compatibility: CompatibilityMetadata{
			InputArtifactTypeKeys:  []string{"harpia.artifacts.v1.LinkedInPostDraft"},
			OutputArtifactTypeKeys: []string{"harpia.artifacts.v1.PublishConfirmation"},
			ConnectionType:         "oauth_linkedin",
		},
	},
	{
		Key:         SKUNewsletterWriterSenior,
		DisplayName: "Newsletter Writer (Senior)",
		Description: "Senior agent that synthesizes a platform-neutral newsletter draft from curated news.",
		Kind:        KindAgent,
		PriceCents:  200,
		Currency:    "USD",
		Compatibility: CompatibilityMetadata{
			InputArtifactTypeKeys:  []string{"harpia.artifacts.v1.NewsList"},
			OutputArtifactTypeKeys: []string{"harpia.artifacts.v1.TextDraft"},
			ManifestID:             "newsletter-writer-senior",
			ManifestVersion:        "1.0.0",
		},
	},
	{
		Key:         SKULinkedInVoiceSenior,
		DisplayName: "LinkedIn Voice (Senior)",
		Description: "Senior agent that adapts a neutral text draft into a LinkedIn-ready post.",
		Kind:        KindAgent,
		PriceCents:  150,
		Currency:    "USD",
		Compatibility: CompatibilityMetadata{
			InputArtifactTypeKeys:  []string{"harpia.artifacts.v1.TextDraft"},
			OutputArtifactTypeKeys: []string{"harpia.artifacts.v1.LinkedInPostDraft"},
			ManifestID:             "linkedin-voice-senior",
			ManifestVersion:        "1.0.0",
		},
	},
}

func EnsureCatalog(ctx context.Context, pool *pgxpool.Pool) error {
	repo := NewRepository(pool)
	for _, seed := range DefaultCatalogSeeds {
		if _, err := repo.UpsertSKU(ctx, &ExecutorSKU{
			Key:           seed.Key,
			DisplayName:   seed.DisplayName,
			Description:   seed.Description,
			Kind:          seed.Kind,
			PriceCents:    seed.PriceCents,
			Currency:      seed.Currency,
			Compatibility: seed.Compatibility,
		}); err != nil {
			return fmt.Errorf("seed executor sku %q: %w", seed.Key, err)
		}
	}
	return nil
}

func EnsureDevEntitlements(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID) error {
	repo := NewRepository(pool)
	for _, seed := range DefaultCatalogSeeds {
		sku, err := repo.GetSKUByKey(ctx, seed.Key)
		if err != nil {
			return fmt.Errorf("load executor sku %q: %w", seed.Key, err)
		}
		if _, err := repo.CreateEntitlement(ctx, tenantID, sku.ID, nil); err != nil {
			return fmt.Errorf("seed entitlement for %q: %w", seed.Key, err)
		}
	}
	return nil
}
