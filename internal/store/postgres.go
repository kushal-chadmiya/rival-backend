package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"taskboard-backend/internal/tasks"
)

// PostgresStore persists tasks in PostgreSQL.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// New creates a new Postgres-backed task store.
func New(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

// CreateTask inserts a task into PostgreSQL.
func (s *PostgresStore) CreateTask(ctx context.Context, params tasks.CreateTaskParams) (tasks.Task, error) {
	const query = `
		insert into tasks (user_id, title, description, status, priority, due_date)
		values ($1, $2, $3, $4, $5, $6)
		returning id, user_id, title, description, status, priority, due_date, created_at, updated_at
	`

	var task tasks.Task
	err := s.pool.QueryRow(
		ctx,
		query,
		params.UserID,
		params.Title,
		params.Description,
		params.Status,
		params.Priority,
		params.DueDate,
	).Scan(
		&task.ID,
		&task.UserID,
		&task.Title,
		&task.Description,
		&task.Status,
		&task.Priority,
		&task.DueDate,
		&task.CreatedAt,
		&task.UpdatedAt,
	)
	if err != nil {
		return tasks.Task{}, fmt.Errorf("insert task: %w", err)
	}

	return task, nil
}

// ListTasks lists paginated tasks for a user or for admins.
func (s *PostgresStore) ListTasks(ctx context.Context, userID string, isAdmin bool, params tasks.ListParams) (tasks.ListResult, error) {
	var whereParts []string
	var args []any
	argPos := 1

	if !isAdmin {
		whereParts = append(whereParts, fmt.Sprintf("user_id = $%d", argPos))
		args = append(args, userID)
		argPos++
	}
	if params.Status != "" {
		whereParts = append(whereParts, fmt.Sprintf("status = $%d", argPos))
		args = append(args, params.Status)
		argPos++
	}
	if params.Search != "" {
		whereParts = append(whereParts, fmt.Sprintf("title ilike $%d", argPos))
		args = append(args, "%"+params.Search+"%")
		argPos++
	}

	whereClause := ""
	if len(whereParts) > 0 {
		whereClause = "where " + strings.Join(whereParts, " and ")
	}

	var total int
	countQuery := "select count(*) from tasks " + whereClause
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return tasks.ListResult{}, fmt.Errorf("count tasks: %w", err)
	}

	orderBy := map[string]string{
		"created_at": "created_at",
		"due_date":   "due_date",
		"priority":   `case priority when 'high' then 3 when 'medium' then 2 else 1 end`,
	}[params.SortBy]

	offset := (params.Page - 1) * params.PageSize
	args = append(args, params.PageSize, offset)
	limitPos := argPos
	offsetPos := argPos + 1

	listQuery := fmt.Sprintf(`
		select id, user_id, title, description, status, priority, due_date, created_at, updated_at
		from tasks
		%s
		order by %s %s, created_at desc
		limit $%d offset $%d
	`, whereClause, orderBy, strings.ToUpper(params.SortDir), limitPos, offsetPos)

	rows, err := s.pool.Query(ctx, listQuery, args...)
	if err != nil {
		return tasks.ListResult{}, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	items := make([]tasks.Task, 0, params.PageSize)
	for rows.Next() {
		var task tasks.Task
		if err := rows.Scan(
			&task.ID,
			&task.UserID,
			&task.Title,
			&task.Description,
			&task.Status,
			&task.Priority,
			&task.DueDate,
			&task.CreatedAt,
			&task.UpdatedAt,
		); err != nil {
			return tasks.ListResult{}, fmt.Errorf("scan task: %w", err)
		}
		items = append(items, task)
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + params.PageSize - 1) / params.PageSize
	}

	return tasks.ListResult{
		Items:      items,
		Page:       params.Page,
		PageSize:   params.PageSize,
		Total:      total,
		TotalPages: totalPages,
	}, nil
}

// GetTask fetches a task if the viewer owns it or is an admin.
func (s *PostgresStore) GetTask(ctx context.Context, id string, userID string, isAdmin bool) (tasks.Task, error) {
	query := `
		select id, user_id, title, description, status, priority, due_date, created_at, updated_at
		from tasks
		where id = $1
	`
	args := []any{id}

	if !isAdmin {
		query += " and user_id = $2"
		args = append(args, userID)
	}

	var task tasks.Task
	err := s.pool.QueryRow(ctx, query, args...).Scan(
		&task.ID,
		&task.UserID,
		&task.Title,
		&task.Description,
		&task.Status,
		&task.Priority,
		&task.DueDate,
		&task.CreatedAt,
		&task.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tasks.Task{}, tasks.ErrNotFound
		}
		return tasks.Task{}, fmt.Errorf("get task: %w", err)
	}

	return task, nil
}

// UpdateTask applies partial task updates.
func (s *PostgresStore) UpdateTask(ctx context.Context, params tasks.UpdateTaskParams) (tasks.Task, error) {
	updates := []string{}
	args := []any{}
	position := 1

	if params.Title != nil {
		updates = append(updates, fmt.Sprintf("title = $%d", position))
		args = append(args, *params.Title)
		position++
	}
	if params.Description != nil {
		updates = append(updates, fmt.Sprintf("description = $%d", position))
		args = append(args, *params.Description)
		position++
	}
	if params.Status != nil {
		updates = append(updates, fmt.Sprintf("status = $%d", position))
		args = append(args, *params.Status)
		position++
	}
	if params.Priority != nil {
		updates = append(updates, fmt.Sprintf("priority = $%d", position))
		args = append(args, *params.Priority)
		position++
	}
	if params.DueDate != nil {
		updates = append(updates, fmt.Sprintf("due_date = $%d", position))
		args = append(args, *params.DueDate)
		position++
	}

	updates = append(updates, "updated_at = now()")
	args = append(args, params.ID)
	idPos := position
	position++

	conditions := []string{fmt.Sprintf("id = $%d", idPos)}
	if !params.IsAdmin {
		args = append(args, params.UserID)
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", position))
	}

	query := fmt.Sprintf(`
		update tasks
		set %s
		where %s
		returning id, user_id, title, description, status, priority, due_date, created_at, updated_at
	`, strings.Join(updates, ", "), strings.Join(conditions, " and "))

	var task tasks.Task
	err := s.pool.QueryRow(ctx, query, args...).Scan(
		&task.ID,
		&task.UserID,
		&task.Title,
		&task.Description,
		&task.Status,
		&task.Priority,
		&task.DueDate,
		&task.CreatedAt,
		&task.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tasks.Task{}, tasks.ErrNotFound
		}
		return tasks.Task{}, fmt.Errorf("update task: %w", err)
	}

	return task, nil
}

// DeleteTask removes a task if the viewer owns it or is an admin.
func (s *PostgresStore) DeleteTask(ctx context.Context, id string, userID string, isAdmin bool) error {
	query := "delete from tasks where id = $1"
	args := []any{id}
	if !isAdmin {
		query += " and user_id = $2"
		args = append(args, userID)
	}

	commandTag, err := s.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return tasks.ErrNotFound
	}

	return nil
}

// ParseUUID normalizes task IDs before sending them to PostgreSQL.
func ParseUUID(id string) (uuid.UUID, error) {
	return uuid.Parse(id)
}
