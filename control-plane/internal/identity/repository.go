package identity

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MembershipResolver interface {
	ResolveMemberships(ctx context.Context, user AuthenticatedUser) ([]TenantMembership, error)
}

type MembershipRepositoryOptions struct {
	DefaultTenantID            uuid.UUID
	AutoProvisionDefaultTenant bool
}

type MembershipRepository struct {
	pool            *pgxpool.Pool
	defaultTenantID uuid.UUID
	autoProvision   bool
}

func NewMembershipRepository(pool *pgxpool.Pool, opts MembershipRepositoryOptions) *MembershipRepository {
	return &MembershipRepository{
		pool:            pool,
		defaultTenantID: opts.DefaultTenantID,
		autoProvision:   opts.AutoProvisionDefaultTenant,
	}
}

func (r *MembershipRepository) ResolveMemberships(ctx context.Context, user AuthenticatedUser) ([]TenantMembership, error) {
	memberships, err := r.listMemberships(ctx, user.Subject)
	if err != nil {
		return nil, err
	}
	if len(memberships) > 0 || !r.autoProvision || r.defaultTenantID == uuid.Nil {
		return memberships, nil
	}

	if err := r.provisionDefaultMembership(ctx, user); err != nil {
		return nil, err
	}
	memberships, err = r.listMemberships(ctx, user.Subject)
	if err != nil {
		return nil, err
	}
	return memberships, nil
}

func (r *MembershipRepository) listMemberships(ctx context.Context, externalID string) ([]TenantMembership, error) {
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

func (r *MembershipRepository) provisionDefaultMembership(ctx context.Context, user AuthenticatedUser) error {
	email := firstNonEmpty(user.Email, user.Subject)
	name := firstNonEmpty(user.Name, user.Email, user.Subject)
	_, err := r.pool.Exec(ctx,
		`INSERT INTO users (tenant_id, external_id, email, name, role)
		 VALUES ($1, $2, $3, $4, 'member')
		 ON CONFLICT (tenant_id, external_id) DO NOTHING`,
		r.defaultTenantID,
		user.Subject,
		email,
		name,
	)
	if err != nil {
		return fmt.Errorf("provision default membership: %w", err)
	}
	return nil
}
