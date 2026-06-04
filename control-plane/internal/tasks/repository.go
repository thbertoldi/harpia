package tasks

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/harpia/control-plane/internal/database"
)

type Task struct {
	ID            uuid.UUID       `json:"id"`
	TenantID      uuid.UUID       `json:"tenant_id"`
	Title         string          `json:"title"`
	Description   string          `json:"description"`
	Status        string          `json:"status"`
	Priority      int             `json:"priority"`
	AgentTypeID   uuid.NullUUID   `json:"agent_type_id"`
	Context       json.RawMessage `json:"context"`
	Result        json.RawMessage `json:"result"`
	Error         string          `json:"error"`
	CreatedBy     uuid.UUID       `json:"created_by"`
	CreatedByName string          `json:"created_by_name"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	CompletedAt   sql.NullTime    `json:"completed_at"`
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, task *Task) (*Task, error) {
	conn, err := database.SetTenantContext(ctx, r.pool, task.TenantID)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	row := conn.QueryRow(ctx,
		`INSERT INTO tasks (tenant_id, title, description, status, priority, agent_type_id, context, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, tenant_id, title, description, status, priority, agent_type_id, context, result, error, created_by, created_at, updated_at, completed_at`,
		task.TenantID, task.Title, task.Description, task.Status, task.Priority,
		task.AgentTypeID, task.Context, task.CreatedBy,
	)

	var created Task
	err = row.Scan(
		&created.ID, &created.TenantID, &created.Title, &created.Description,
		&created.Status, &created.Priority, &created.AgentTypeID, &created.Context,
		&created.Result, &created.Error, &created.CreatedBy,
		&created.CreatedAt, &created.UpdatedAt, &created.CompletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert task: %w", err)
	}

	created.CreatedByName = r.fetchUserName(ctx, conn, created.CreatedBy)
	return &created, nil
}

func (r *Repository) GetByID(ctx context.Context, tenantID, taskID uuid.UUID) (*Task, error) {
	conn, err := database.SetTenantContext(ctx, r.pool, tenantID)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	row := conn.QueryRow(ctx,
		`SELECT t.id, t.tenant_id, t.title, t.description, t.status, t.priority,
		        t.agent_type_id, t.context, t.result, t.error, t.created_by,
		        COALESCE(u.name, '') AS created_by_name,
		        t.created_at, t.updated_at, t.completed_at
		 FROM tasks t
		 LEFT JOIN users u ON u.id = t.created_by
		 WHERE t.id = $1 AND t.tenant_id = $2`,
		taskID, tenantID,
	)

	var task Task
	err = row.Scan(
		&task.ID, &task.TenantID, &task.Title, &task.Description,
		&task.Status, &task.Priority, &task.AgentTypeID, &task.Context,
		&task.Result, &task.Error, &task.CreatedBy, &task.CreatedByName,
		&task.CreatedAt, &task.UpdatedAt, &task.CompletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}

	return &task, nil
}

func (r *Repository) List(ctx context.Context, tenantID uuid.UUID, status string, limit, offset int) ([]Task, error) {
	conn, err := database.SetTenantContext(ctx, r.pool, tenantID)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	var rows pgx.Rows
	if status != "" {
		rows, err = conn.Query(ctx,
			`SELECT t.id, t.tenant_id, t.title, t.description, t.status, t.priority,
			        t.agent_type_id, t.context, t.result, t.error, t.created_by,
			        COALESCE(u.name, '') AS created_by_name,
			        t.created_at, t.updated_at, t.completed_at
			 FROM tasks t
			 LEFT JOIN users u ON u.id = t.created_by
			 WHERE t.tenant_id = $1 AND t.status = $2
			 ORDER BY t.created_at DESC
			 LIMIT $3 OFFSET $4`,
			tenantID, status, limit, offset,
		)
	} else {
		rows, err = conn.Query(ctx,
			`SELECT t.id, t.tenant_id, t.title, t.description, t.status, t.priority,
			        t.agent_type_id, t.context, t.result, t.error, t.created_by,
			        COALESCE(u.name, '') AS created_by_name,
			        t.created_at, t.updated_at, t.completed_at
			 FROM tasks t
			 LEFT JOIN users u ON u.id = t.created_by
			 WHERE t.tenant_id = $1
			 ORDER BY t.created_at DESC
			 LIMIT $2 OFFSET $3`,
			tenantID, limit, offset,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]Task, 0)
	for rows.Next() {
		var task Task
		if err := rows.Scan(
			&task.ID, &task.TenantID, &task.Title, &task.Description,
			&task.Status, &task.Priority, &task.AgentTypeID, &task.Context,
			&task.Result, &task.Error, &task.CreatedBy, &task.CreatedByName,
			&task.CreatedAt, &task.UpdatedAt, &task.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tasks: %w", err)
	}

	return tasks, nil
}

func (r *Repository) UpdateStatus(ctx context.Context, tenantID, taskID uuid.UUID, status string) error {
	conn, err := database.SetTenantContext(ctx, r.pool, tenantID)
	if err != nil {
		return err
	}
	defer conn.Release()

	tag, err := conn.Exec(ctx,
		`UPDATE tasks SET status = $1, updated_at = now() WHERE id = $2`,
		status, taskID,
	)
	if err != nil {
		return fmt.Errorf("update task status: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("task not found: %s", taskID)
	}

	return nil
}

func (r *Repository) UpdateResult(ctx context.Context, tenantID, taskID uuid.UUID, result json.RawMessage) error {
	conn, err := database.SetTenantContext(ctx, r.pool, tenantID)
	if err != nil {
		return err
	}
	defer conn.Release()

	tag, err := conn.Exec(ctx,
		`UPDATE tasks SET result = $1, updated_at = now() WHERE id = $2`,
		result, taskID,
	)
	if err != nil {
		return fmt.Errorf("update task result: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("task not found: %s", taskID)
	}

	return nil
}

func (r *Repository) fetchUserName(ctx context.Context, conn *pgxpool.Conn, userID uuid.UUID) string {
	var name string
	if err := conn.QueryRow(ctx, `SELECT COALESCE(name, '') FROM users WHERE id = $1`, userID).Scan(&name); err != nil {
		return ""
	}
	return name
}
