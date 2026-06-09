package identity

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type MembershipResolver interface {
	ListMemberships(ctx context.Context, externalID string) ([]TenantMembership, error)
}

type MembershipRepository struct {
	pool *pgxpool.Pool
}

func NewMembershipRepository(pool *pgxpool.Pool) *MembershipRepository {
	return &MembershipRepository{pool: pool}
}

func (r *MembershipRepository) ListMemberships(ctx context.Context, externalID string) ([]TenantMembership, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT t.id, t.slug, t.name, u.role
		 FROM users u
		 JOIN tenants t ON t.id = u.tenant_id
		 WHERE u.external_id = $1
		 ORDER BY t.name ASC`,
		externalID,
	)
	if err != nil {
		return nil, fmt.Errorf("list memberships: %w", err)
	}
	defer rows.Close()

	memberships := make([]TenantMembership, 0)
	for rows.Next() {
		var membership TenantMembership
		if err := rows.Scan(
			&membership.TenantID,
			&membership.Slug,
			&membership.Name,
			&membership.Role,
		); err != nil {
			return nil, fmt.Errorf("scan membership: %w", err)
		}
		memberships = append(memberships, membership)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memberships: %w", err)
	}
	return memberships, nil
}
