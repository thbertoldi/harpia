package executors

import (
	"context"
	"encoding/json"
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
			ManifestVersion:        "0.1.0",
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
			ManifestVersion:        "0.1.0",
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

// EnsureTenantAgentInstallations provisions default agent executor installations for
// entitled agent SKUs when none exist yet. Integrations remain user-configured.
func EnsureTenantAgentInstallations(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID) error {
	repo := NewRepository(pool)
	entitlements, err := repo.ListEntitlements(ctx, tenantID, nil, 100, 0)
	if err != nil {
		return fmt.Errorf("list entitlements: %w", err)
	}

	for _, entitlement := range entitlements {
		sku, err := repo.GetSKUByID(ctx, entitlement.ExecutorSKUID)
		if err != nil {
			return fmt.Errorf("load sku for entitlement: %w", err)
		}
		if sku.Kind != KindAgent {
			continue
		}

		existing, err := repo.ListInstallations(ctx, tenantID, KindAgent, &sku.ID, 1, 0)
		if err != nil {
			return fmt.Errorf("list agent installations for %q: %w", sku.Key, err)
		}
		if len(existing) > 0 {
			continue
		}

		manifestID := sku.Compatibility.ManifestID
		manifestVersion := sku.Compatibility.ManifestVersion
		if manifestID == "" || manifestVersion == "" {
			return fmt.Errorf("agent sku %q is missing manifest metadata", sku.Key)
		}

		_, err = repo.CreateInstallation(ctx, &ExecutorInstallation{
			TenantID:        tenantID,
			ExecutorSKUID:   sku.ID,
			Kind:            KindAgent,
			DisplayName:     sku.DisplayName,
			Enabled:         true,
			ConfigJSON:      json.RawMessage("{}"),
			ManifestID:      &manifestID,
			ManifestVersion: &manifestVersion,
		})
		if err != nil {
			return fmt.Errorf("create agent installation for %q: %w", sku.Key, err)
		}
	}

	return nil
}
