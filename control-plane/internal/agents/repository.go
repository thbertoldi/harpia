package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"

	"github.com/harpia/control-plane/internal/database"
)

type AgentType struct {
	ID           uuid.UUID       `json:"id"`
	TenantID     uuid.UUID       `json:"tenant_id"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	InputSchema  json.RawMessage `json:"input_schema"`
	OutputSchema json.RawMessage `json:"output_schema"`
	MCPServers   json.RawMessage `json:"mcp_servers"`
	Enabled      bool            `json:"enabled"`
	CreatedAt    time.Time       `json:"created_at"`
	Similarity   float32         `json:"similarity,omitempty"`
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, tenantID uuid.UUID, agentType *AgentType) (*AgentType, error) {
	var created AgentType
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		row := q.QueryRow(ctx,
			`INSERT INTO agent_types (tenant_id, name, description, input_schema, output_schema, mcp_servers, enabled)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)
			 RETURNING id, tenant_id, name, description, input_schema, output_schema, mcp_servers, enabled, created_at`,
			tenantID, agentType.Name, agentType.Description, agentType.InputSchema,
			agentType.OutputSchema, agentType.MCPServers, agentType.Enabled,
		)

		return row.Scan(
			&created.ID, &created.TenantID, &created.Name, &created.Description,
			&created.InputSchema, &created.OutputSchema, &created.MCPServers,
			&created.Enabled, &created.CreatedAt,
		)
	})
	if err != nil {
		return nil, fmt.Errorf("insert agent type: %w", err)
	}

	return &created, nil
}

func (r *Repository) UpsertByName(ctx context.Context, tenantID uuid.UUID, agentType *AgentType) (*AgentType, error) {
	var upserted AgentType
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		row := q.QueryRow(ctx,
			`INSERT INTO agent_types (tenant_id, name, description, input_schema, output_schema, mcp_servers, enabled)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)
			 ON CONFLICT (tenant_id, name) DO UPDATE SET
			   description = EXCLUDED.description,
			   enabled = EXCLUDED.enabled
			 RETURNING id, tenant_id, name, description, input_schema, output_schema, mcp_servers, enabled, created_at`,
			tenantID, agentType.Name, agentType.Description, agentType.InputSchema,
			agentType.OutputSchema, agentType.MCPServers, agentType.Enabled,
		)

		return row.Scan(
			&upserted.ID, &upserted.TenantID, &upserted.Name, &upserted.Description,
			&upserted.InputSchema, &upserted.OutputSchema, &upserted.MCPServers,
			&upserted.Enabled, &upserted.CreatedAt,
		)
	})
	if err != nil {
		return nil, fmt.Errorf("upsert agent type: %w", err)
	}

	return &upserted, nil
}

func (r *Repository) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*AgentType, error) {
	var agentType AgentType
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		row := q.QueryRow(ctx,
			`SELECT id, tenant_id, name, description, input_schema, output_schema, mcp_servers, enabled, created_at
			 FROM agent_types
			 WHERE id = $1 AND tenant_id = $2`,
			id, tenantID,
		)

		return row.Scan(
			&agentType.ID, &agentType.TenantID, &agentType.Name, &agentType.Description,
			&agentType.InputSchema, &agentType.OutputSchema, &agentType.MCPServers,
			&agentType.Enabled, &agentType.CreatedAt,
		)
	})
	if err != nil {
		return nil, fmt.Errorf("get agent type: %w", err)
	}

	return &agentType, nil
}

func (r *Repository) List(ctx context.Context, tenantID uuid.UUID, enabledOnly bool) ([]AgentType, error) {
	agentTypes := make([]AgentType, 0)
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		var rows interface {
			Close()
			Err() error
			Next() bool
			Scan(dest ...any) error
		}
		var err error

		if enabledOnly {
			rows, err = q.Query(ctx,
				`SELECT id, tenant_id, name, description, input_schema, output_schema, mcp_servers, enabled, created_at
			 FROM agent_types
			 WHERE tenant_id = $1 AND enabled = true
			 ORDER BY name ASC`,
				tenantID,
			)
		} else {
			rows, err = q.Query(ctx,
				`SELECT id, tenant_id, name, description, input_schema, output_schema, mcp_servers, enabled, created_at
			 FROM agent_types
			 WHERE tenant_id = $1
			 ORDER BY name ASC`,
				tenantID,
			)
		}
		if err != nil {
			return fmt.Errorf("list agent types: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var agentType AgentType
			if err := rows.Scan(
				&agentType.ID, &agentType.TenantID, &agentType.Name, &agentType.Description,
				&agentType.InputSchema, &agentType.OutputSchema, &agentType.MCPServers,
				&agentType.Enabled, &agentType.CreatedAt,
			); err != nil {
				return fmt.Errorf("scan agent type: %w", err)
			}
			agentTypes = append(agentTypes, agentType)
		}

		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate agent types: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return agentTypes, nil
}

func (r *Repository) MatchByCapability(ctx context.Context, tenantID uuid.UUID, embedding []float32, limit int) ([]AgentType, error) {
	agentTypes := make([]AgentType, 0)
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		rows, err := q.Query(ctx,
			`SELECT id, tenant_id, name, description, input_schema, output_schema, mcp_servers, enabled, created_at,
		        1 - (capabilities_embedding <=> $1) AS similarity
		 FROM agent_types
		 WHERE tenant_id = $2 AND enabled = true
		 ORDER BY capabilities_embedding <=> $1
		 LIMIT $3`,
			pgvector.NewVector(embedding),
			tenantID,
			limit,
		)
		if err != nil {
			return fmt.Errorf("match by capability: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var agentType AgentType
			if err := rows.Scan(
				&agentType.ID, &agentType.TenantID, &agentType.Name, &agentType.Description,
				&agentType.InputSchema, &agentType.OutputSchema, &agentType.MCPServers,
				&agentType.Enabled, &agentType.CreatedAt,
				&agentType.Similarity,
			); err != nil {
				return fmt.Errorf("scan agent match: %w", err)
			}
			agentTypes = append(agentTypes, agentType)
		}

		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate agent matches: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return agentTypes, nil
}

func (r *Repository) SetEmbedding(ctx context.Context, tenantID, id uuid.UUID, embedding []float32) error {
	return database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		_, err := q.Exec(ctx,
			`UPDATE agent_types
			 SET capabilities_embedding = $1
			 WHERE id = $2 AND tenant_id = $3`,
			pgvector.NewVector(embedding),
			id,
			tenantID,
		)
		if err != nil {
			return fmt.Errorf("set embedding: %w", err)
		}
		return nil
	})
}

func (r *Repository) Update(ctx context.Context, tenantID uuid.UUID, agentType *AgentType) (*AgentType, error) {
	var updated AgentType
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		row := q.QueryRow(ctx,
			`UPDATE agent_types
			 SET name = $1, description = $2, input_schema = $3, output_schema = $4, mcp_servers = $5, enabled = $6
			 WHERE id = $7 AND tenant_id = $8
			 RETURNING id, tenant_id, name, description, input_schema, output_schema, mcp_servers, enabled, created_at`,
			agentType.Name, agentType.Description, agentType.InputSchema,
			agentType.OutputSchema, agentType.MCPServers, agentType.Enabled,
			agentType.ID, tenantID,
		)

		return row.Scan(
			&updated.ID, &updated.TenantID, &updated.Name, &updated.Description,
			&updated.InputSchema, &updated.OutputSchema, &updated.MCPServers,
			&updated.Enabled, &updated.CreatedAt,
		)
	})
	if err != nil {
		return nil, fmt.Errorf("update agent type: %w", err)
	}

	return &updated, nil
}
