package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AgentType struct {
	ID           uuid.UUID       `json:"id"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	InputSchema  json.RawMessage `json:"input_schema"`
	OutputSchema json.RawMessage `json:"output_schema"`
	MCPServers   json.RawMessage `json:"mcp_servers"`
	Enabled      bool            `json:"enabled"`
	CreatedAt    time.Time       `json:"created_at"`
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, agentType *AgentType) (*AgentType, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO agent_types (name, description, input_schema, output_schema, mcp_servers, enabled)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, name, description, input_schema, output_schema, mcp_servers, enabled, created_at`,
		agentType.Name, agentType.Description, agentType.InputSchema,
		agentType.OutputSchema, agentType.MCPServers, agentType.Enabled,
	)

	var created AgentType
	err := row.Scan(
		&created.ID, &created.Name, &created.Description,
		&created.InputSchema, &created.OutputSchema, &created.MCPServers,
		&created.Enabled, &created.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert agent type: %w", err)
	}

	return &created, nil
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*AgentType, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, name, description, input_schema, output_schema, mcp_servers, enabled, created_at
		 FROM agent_types
		 WHERE id = $1`,
		id,
	)

	var agentType AgentType
	err := row.Scan(
		&agentType.ID, &agentType.Name, &agentType.Description,
		&agentType.InputSchema, &agentType.OutputSchema, &agentType.MCPServers,
		&agentType.Enabled, &agentType.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get agent type: %w", err)
	}

	return &agentType, nil
}

func (r *Repository) List(ctx context.Context, enabledOnly bool) ([]AgentType, error) {
	var rows interface {
		Close()
		Err() error
		Next() bool
		Scan(dest ...any) error
	}
	var err error

	if enabledOnly {
		rows, err = r.pool.Query(ctx,
			`SELECT id, name, description, input_schema, output_schema, mcp_servers, enabled, created_at
			 FROM agent_types
			 WHERE enabled = true
			 ORDER BY name ASC`,
		)
	} else {
		rows, err = r.pool.Query(ctx,
			`SELECT id, name, description, input_schema, output_schema, mcp_servers, enabled, created_at
			 FROM agent_types
			 ORDER BY name ASC`,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("list agent types: %w", err)
	}
	defer rows.Close()

	agentTypes := make([]AgentType, 0)
	for rows.Next() {
		var agentType AgentType
		if err := rows.Scan(
			&agentType.ID, &agentType.Name, &agentType.Description,
			&agentType.InputSchema, &agentType.OutputSchema, &agentType.MCPServers,
			&agentType.Enabled, &agentType.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan agent type: %w", err)
		}
		agentTypes = append(agentTypes, agentType)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agent types: %w", err)
	}

	return agentTypes, nil
}

func (r *Repository) Update(ctx context.Context, agentType *AgentType) (*AgentType, error) {
	row := r.pool.QueryRow(ctx,
		`UPDATE agent_types
		 SET name = $1, description = $2, input_schema = $3, output_schema = $4, mcp_servers = $5, enabled = $6
		 WHERE id = $7
		 RETURNING id, name, description, input_schema, output_schema, mcp_servers, enabled, created_at`,
		agentType.Name, agentType.Description, agentType.InputSchema,
		agentType.OutputSchema, agentType.MCPServers, agentType.Enabled,
		agentType.ID,
	)

	var updated AgentType
	err := row.Scan(
		&updated.ID, &updated.Name, &updated.Description,
		&updated.InputSchema, &updated.OutputSchema, &updated.MCPServers,
		&updated.Enabled, &updated.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update agent type: %w", err)
	}

	return &updated, nil
}
