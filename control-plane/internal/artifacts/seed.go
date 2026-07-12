package artifacts

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ArtifactTypeSeed is a single declarative artifact-type row to ensure at boot.
// Catalog data stays in Go, not in migrations (Platform Constitution §13:
// "Schema stays in migrations; [seed] data does not").
type ArtifactTypeSeed struct {
	Key         string
	SchemaRef   string
	Description string
	Version     int
}

// DefaultArtifactTypeSeeds covers every artifact type referenced by plan
// templates and executor SKUs. Keys reuse the TypeKey* constants from
// validation.go so the seed list and the payload validator stay in sync.
var DefaultArtifactTypeSeeds = []ArtifactTypeSeed{
	{
		Key:         TypeKeyDateRange,
		SchemaRef:   "harpia.artifacts.v1/DateRange",
		Version:     1,
		Description: "Inclusive date range for plan seed inputs.",
	},
	{
		Key:         TypeKeyNewsList,
		SchemaRef:   "harpia.artifacts.v1/NewsList",
		Version:     1,
		Description: "Curated news articles collected for a plan step.",
	},
	{
		Key:         TypeKeyTextDraft,
		SchemaRef:   "harpia.artifacts.v1/TextDraft",
		Version:     1,
		Description: "Platform-neutral text draft.",
	},
	{
		Key:         TypeKeyLinkedInPostDraft,
		SchemaRef:   "harpia.artifacts.v1/LinkedInPostDraft",
		Version:     1,
		Description: "LinkedIn-specific post draft.",
	},
	{
		Key:         TypeKeyLinkedInPost,
		SchemaRef:   "harpia.artifacts.v1/LinkedInPost",
		Version:     1,
		Description: "Composable versioned LinkedIn post with optional carousel and images.",
	},
	{
		Key:         TypeKeyLinkedInCarouselDocument,
		SchemaRef:   "harpia.artifacts.v1/LinkedInCarouselDocument",
		Version:     1,
		Description: "Immutable PDF derivative for a LinkedIn carousel.",
	},
	{
		Key:         TypeKeyPublishConfirmation,
		SchemaRef:   "harpia.artifacts.v1/PublishConfirmation",
		Version:     1,
		Description: "Confirmation payload after publishing to an external platform.",
	},
	{
		Key:         TypeKeyCarouselDraft,
		SchemaRef:   "harpia.artifacts.v1/CarouselDraft",
		Version:     1,
		Description: "LinkedIn carousel draft (slide outline).",
	},
	{
		Key:         TypeKeyImageAsset,
		SchemaRef:   "harpia.artifacts.v1/ImageAsset",
		Version:     1,
		Description: "Generated image asset with provenance prompt.",
	},
}

// EnsureArtifactTypes idempotently upserts every default artifact type. It must
// run before plan templates are seeded, since templates validate their input and
// output artifact type keys against this table.
func EnsureArtifactTypes(ctx context.Context, pool *pgxpool.Pool) error {
	repo := NewRepository(pool)
	for _, seed := range DefaultArtifactTypeSeeds {
		if _, err := repo.UpsertType(ctx, &ArtifactType{
			Key:         seed.Key,
			SchemaRef:   seed.SchemaRef,
			Version:     int32(seed.Version),
			Description: seed.Description,
		}); err != nil {
			return fmt.Errorf("seed artifact type %q: %w", seed.Key, err)
		}
	}
	return nil
}
