package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"taskboard-backend/internal/auth"
)

const (
	ActionCreated            = "created"
	ActionUpdated            = "updated"
	ActionDeleted            = "deleted"
	ActionAttachmentAdded    = "attachment_added"
	ActionAttachmentRemoved  = "attachment_removed"
)

// Entry is a persisted activity log row.
type Entry struct {
	ID         uuid.UUID       `json:"id"`
	TaskID     uuid.UUID       `json:"task_id"`
	ActorID    string          `json:"actor_id"`
	ActorEmail string          `json:"actor_email"`
	Action     string          `json:"action"`
	Changes    json.RawMessage `json:"changes"`
	CreatedAt  time.Time       `json:"created_at"`
}

// Store persists task activity logs.
type Store struct {
	pool *pgxpool.Pool
}

// New creates an activity store.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Record inserts a new activity entry.
func (s *Store) Record(ctx context.Context, taskID string, actor auth.Viewer, action string, changes map[string]any) error {
	payload, err := json.Marshal(changes)
	if err != nil {
		return fmt.Errorf("marshal activity changes: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		insert into task_activity (task_id, actor_id, actor_email, action, changes)
		values ($1, $2, $3, $4, $5)
	`, taskID, actor.UserID, actor.Email, action, payload)
	if err != nil {
		return fmt.Errorf("insert activity: %w", err)
	}

	return nil
}

// ListByTask returns activity for a task if the viewer can access it.
func (s *Store) ListByTask(ctx context.Context, taskID string, userID string, isAdmin bool) ([]Entry, error) {
	query := `
		select a.id, a.task_id, a.actor_id, a.actor_email, a.action, a.changes, a.created_at
		from task_activity a
		join tasks t on t.id = a.task_id
		where a.task_id = $1
	`
	args := []any{taskID}
	if !isAdmin {
		query += " and t.user_id = $2"
		args = append(args, userID)
	}
	query += " order by a.created_at desc limit 100"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list activity: %w", err)
	}
	defer rows.Close()

	items := make([]Entry, 0)
	for rows.Next() {
		var entry Entry
		if err := rows.Scan(
			&entry.ID,
			&entry.TaskID,
			&entry.ActorID,
			&entry.ActorEmail,
			&entry.Action,
			&entry.Changes,
			&entry.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan activity: %w", err)
		}
		items = append(items, entry)
	}

	return items, nil
}
