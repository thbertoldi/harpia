package executors

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/harpia/control-plane/internal/database"
)

func TestEnsureTenantRSSPresetInstallationsCreatesAndUpdates(t *testing.T) {
	databaseURL := os.Getenv("HARPIA_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set HARPIA_TEST_DATABASE_URL to run RSS preset seeder integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := database.NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	tenantID := uuid.New()
	if _, err := tx.Exec(ctx, `INSERT INTO tenants (id, name, slug) VALUES ($1, 'RSS Preset Test', $2)`, tenantID, "rss-preset-test-"+strings.ReplaceAll(tenantID.String(), "-", "")); err != nil {
		t.Fatalf("insert tenant: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO executor_skus (key, display_name, description, kind, price_cents, currency, compatibility)
		VALUES ($1, 'RSS News Feed', 'test rss sku', 'integration', 0, 'USD', '{}'::jsonb)
		ON CONFLICT (key) DO UPDATE SET kind = EXCLUDED.kind
	`, SKURSSNewsFeed); err != nil {
		t.Fatalf("seed rss sku: %v", err)
	}

	presets := []RSSPresetSeed{
		{Name: "Tech/startup", Feeds: []string{"https://techcrunch.com/feed/"}},
		{Name: "Business", Feeds: []string{"https://sloanreview.mit.edu/feed/"}},
	}
	if err := ensureTenantRSSPresetInstallations(ctx, tx, tenantID, presets); err != nil {
		t.Fatalf("initial seed: %v", err)
	}

	presets[0].Feeds = []string{"https://www.theverge.com/rss/index.xml"}
	if err := ensureTenantRSSPresetInstallations(ctx, tx, tenantID, presets); err != nil {
		t.Fatalf("second seed: %v", err)
	}

	var count int
	if err := tx.QueryRow(ctx, `
		SELECT count(*)
		FROM executor_installations
		WHERE tenant_id = $1 AND display_name LIKE 'RSS Preset:%'
	`, tenantID).Scan(&count); err != nil {
		t.Fatalf("count presets: %v", err)
	}
	if count != 2 {
		t.Fatalf("preset count = %d, want 2", count)
	}

	var enabled bool
	var status string
	var config []byte
	if err := tx.QueryRow(ctx, `
		SELECT enabled, connection_status, config_json
		FROM executor_installations
		WHERE tenant_id = $1 AND display_name = 'RSS Preset: Tech/startup'
	`, tenantID).Scan(&enabled, &status, &config); err != nil {
		t.Fatalf("load tech preset: %v", err)
	}
	if !enabled || status != "connected" {
		t.Fatalf("enabled/status = %v/%q, want true/connected", enabled, status)
	}
	var parsed struct {
		Feeds []string `json:"feeds"`
	}
	if err := json.Unmarshal(config, &parsed); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if len(parsed.Feeds) != 1 || parsed.Feeds[0] != "https://www.theverge.com/rss/index.xml" {
		t.Fatalf("feeds = %#v, want updated Verge feed", parsed.Feeds)
	}
}
