package database

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestWithTenantRequiresTenantID(t *testing.T) {
	err := WithTenant(context.Background(), nil, uuid.Nil, func(q Querier) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected missing tenant id error")
	}
}

func TestWithTenantAppliesRLS(t *testing.T) {
	databaseURL := os.Getenv("HARPIA_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set HARPIA_TEST_DATABASE_URL to run PostgreSQL RLS integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer pool.Close()

	tableName := "rls_probe_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	table := pgx.Identifier{tableName}.Sanitize()
	defer func() {
		_, _ = pool.Exec(context.Background(), "DROP TABLE IF EXISTS "+table)
	}()

	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %s (
			tenant_id UUID NOT NULL,
			value TEXT NOT NULL
		)
	`, table)); err != nil {
		t.Fatalf("create probe table: %v", err)
	}

	tenantA := uuid.New()
	tenantB := uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO "+table+" (tenant_id, value) VALUES ($1, 'visible'), ($2, 'hidden')", tenantA, tenantB); err != nil {
		t.Fatalf("seed probe rows: %v", err)
	}

	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		ALTER TABLE %s ENABLE ROW LEVEL SECURITY;
		ALTER TABLE %s FORCE ROW LEVEL SECURITY;
		CREATE POLICY tenant_isolation ON %s
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('harpia.tenant_id', true), '')::UUID)
			WITH CHECK (tenant_id = NULLIF(current_setting('harpia.tenant_id', true), '')::UUID)
	`, table, table, table)); err != nil {
		t.Fatalf("enable probe RLS: %v", err)
	}

	var values []string
	err = WithTenant(ctx, pool, tenantA, func(q Querier) error {
		rows, err := q.Query(ctx, "SELECT value FROM "+table+" ORDER BY value")
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var value string
			if err := rows.Scan(&value); err != nil {
				return err
			}
			values = append(values, value)
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatalf("query with tenant context: %v", err)
	}

	if len(values) != 1 || values[0] != "visible" {
		t.Fatalf("expected only tenant A row, got %#v", values)
	}
}
