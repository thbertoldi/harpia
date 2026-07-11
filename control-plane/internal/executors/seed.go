package executors

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/harpia/control-plane/internal/database"
	"github.com/jackc/pgx/v5"
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

type RSSPresetSeed struct {
	Name  string
	Feeds []string
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
	{
		Key:         SKULinkedinCarouselSenior,
		DisplayName: "LinkedIn Carousel (Senior)",
		Description: "Senior agent that drafts a LinkedIn carousel from a neutral text draft.",
		Kind:        KindAgent,
		PriceCents:  150,
		Currency:    "USD",
		Compatibility: CompatibilityMetadata{
			InputArtifactTypeKeys:  []string{"harpia.artifacts.v1.TextDraft"},
			OutputArtifactTypeKeys: []string{"harpia.artifacts.v1.CarouselDraft"},
			ManifestID:             "linkedin-carousel-senior",
			ManifestVersion:        "0.1.0",
		},
	},
	{
		Key:         SKULinkedinContentSpecialist,
		DisplayName: "LinkedIn Content Specialist (Sênior)",
		Description: "Multi-capable LinkedIn content agent — adapts a text draft into a LinkedIn post or authors a carousel outline.",
		Kind:        KindAgent,
		PriceCents:  200,
		Currency:    "USD",
		Compatibility: CompatibilityMetadata{
			InputArtifactTypeKeys:  []string{"harpia.artifacts.v1.TextDraft"},
			OutputArtifactTypeKeys: []string{"harpia.artifacts.v1.LinkedInPostDraft", "harpia.artifacts.v1.CarouselDraft"},
			ManifestID:             "linkedin-content-specialist",
			ManifestVersion:        "0.1.0",
			Capabilities:           []string{"linkedin-content-adaptation", "carousel-authoring"},
			Tier:                   "senior",
		},
	},
	{
		Key:         SKUImageAssetGenerator,
		DisplayName: "Image Asset Generator",
		Description: "Generates a branded image asset from a content brief via a configured provider.",
		Kind:        KindIntegration,
		PriceCents:  300,
		Currency:    "USD",
		Compatibility: CompatibilityMetadata{
			InputArtifactTypeKeys:  []string{"harpia.artifacts.v1.TextDraft"},
			OutputArtifactTypeKeys: []string{"harpia.artifacts.v1.ImageAsset"},
			ConnectionType:         "image_provider",
			Capabilities:           []string{"image-generation"},
		},
	},
}

var DefaultRSSPresetSeeds = []RSSPresetSeed{
	{
		Name: "Tech/startup",
		Feeds: []string{
			"https://techcrunch.com/feed/",
			"https://www.theverge.com/rss/index.xml",
			"https://feeds.arstechnica.com/arstechnica/index",
			"https://hnrss.org/frontpage",
		},
	},
	{
		Name: "Business",
		Feeds: []string{
			"https://sloanreview.mit.edu/feed/",
		},
	},
	{
		Name: "Marketing/creator",
		Feeds: []string{
			"https://www.socialmediatoday.com/feeds/news/",
		},
	},
	{
		Name: "Brazil/pt-BR",
		Feeds: []string{
			"https://www.infomoney.com.br/feed/",
			"https://exame.com/feed/",
			"https://tecnoblog.net/feed/",
			"https://canaltech.com.br/rss/",
			"https://startupi.com.br/feed/",
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

func EnsureTenantRSSPresetInstallations(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID) error {
	return database.WithTenant(ctx, pool, tenantID, func(q database.Querier) error {
		return ensureTenantRSSPresetInstallations(ctx, q, tenantID, DefaultRSSPresetSeeds)
	})
}

func ensureTenantRSSPresetInstallations(ctx context.Context, q database.Querier, tenantID uuid.UUID, presets []RSSPresetSeed) error {
	if tenantID == uuid.Nil {
		return fmt.Errorf("tenant id is required")
	}
	if _, err := q.Exec(ctx, "SELECT set_config('harpia.tenant_id', $1, true)", tenantID.String()); err != nil {
		return fmt.Errorf("set tenant context: %w", err)
	}

	var skuID uuid.UUID
	if err := q.QueryRow(ctx, `SELECT id FROM executor_skus WHERE key = $1`, SKURSSNewsFeed).Scan(&skuID); err != nil {
		return fmt.Errorf("load rss executor sku: %w", err)
	}

	for _, preset := range presets {
		displayName := "RSS Preset: " + strings.TrimSpace(preset.Name)
		if displayName == "RSS Preset: " {
			return fmt.Errorf("rss preset name is required")
		}
		configJSON, err := json.Marshal(struct {
			Feeds []string `json:"feeds"`
		}{Feeds: preset.Feeds})
		if err != nil {
			return fmt.Errorf("marshal rss preset %q config: %w", preset.Name, err)
		}

		var installationID uuid.UUID
		err = q.QueryRow(ctx, `
			SELECT id
			FROM executor_installations
			WHERE tenant_id = $1 AND display_name = $2
			ORDER BY created_at ASC, id ASC
			LIMIT 1
		`, tenantID, displayName).Scan(&installationID)
		switch {
		case err == nil:
			if _, err := q.Exec(ctx, `
				UPDATE executor_installations
				SET executor_sku_id = $3,
				    kind = $4,
				    enabled = true,
				    connection_status = 'connected',
				    config_json = $5,
				    manifest_id = NULL,
				    manifest_version = NULL,
				    updated_at = now()
				WHERE tenant_id = $1 AND id = $2
			`, tenantID, installationID, skuID, KindIntegration, configJSON); err != nil {
				return fmt.Errorf("update rss preset %q: %w", preset.Name, err)
			}
			if _, err := q.Exec(ctx, `
				DELETE FROM executor_installations
				WHERE tenant_id = $1 AND display_name = $2 AND id <> $3
			`, tenantID, displayName, installationID); err != nil {
				return fmt.Errorf("remove duplicate rss preset %q: %w", preset.Name, err)
			}
		case err == pgx.ErrNoRows:
			if _, err := q.Exec(ctx, `
				INSERT INTO executor_installations (
					tenant_id, executor_sku_id, kind, display_name, enabled,
					connection_status, config_json
				)
				VALUES ($1, $2, $3, $4, true, 'connected', $5)
			`, tenantID, skuID, KindIntegration, displayName, configJSON); err != nil {
				return fmt.Errorf("create rss preset %q: %w", preset.Name, err)
			}
		default:
			return fmt.Errorf("load rss preset %q: %w", preset.Name, err)
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

func EnsureTenantDummyLinkedInInstallation(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID) error {
	return database.WithTenant(ctx, pool, tenantID, func(q database.Querier) error {
		if tenantID == uuid.Nil {
			return fmt.Errorf("tenant id is required")
		}
		if _, err := q.Exec(ctx, "SELECT set_config('harpia.tenant_id', $1, true)", tenantID.String()); err != nil {
			return fmt.Errorf("set tenant context: %w", err)
		}

		var skuID uuid.UUID
		if err := q.QueryRow(ctx, `SELECT id FROM executor_skus WHERE key = $1`, SKULinkedInPublish).Scan(&skuID); err != nil {
			return fmt.Errorf("load linkedin executor sku: %w", err)
		}

		// Always seeds approval_only: OAuth mode requires a real
		// oauth_credential_id (see linkedin.InstallationConfig), and there is no
		// dev-seedable stand-in for a real LinkedIn OAuth credential today. A
		// plan step that specifically requires OAuth mode will not run against
		// this dummy installation until it's reconnected through the real
		// LinkedIn OAuth flow.
		configJSON := []byte(`{"mode": "approval_only"}`)

		_, err := q.Exec(ctx, `
			INSERT INTO executor_installations (
				tenant_id, executor_sku_id, kind, display_name, enabled,
				connection_status, config_json
			) VALUES (
				$1, $2, $3, $4, true, 'connected', $5
			)
			ON CONFLICT (tenant_id, executor_sku_id, display_name) DO NOTHING
		`, tenantID, skuID, KindIntegration, "LinkedIn (Dev Sandbox)", configJSON)
		if err != nil {
			return fmt.Errorf("insert dummy linkedin installation: %w", err)
		}

		return nil
	})
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
