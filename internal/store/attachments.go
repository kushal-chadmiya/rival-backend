package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"taskboard-backend/internal/tasks"
)

// AddAttachment inserts attachment metadata.
func (s *PostgresStore) AddAttachment(ctx context.Context, params tasks.CreateAttachmentParams) (tasks.Attachment, error) {
	const query = `
		insert into task_attachments (task_id, user_id, file_name, mime_type, size_bytes, storage_path)
		values ($1, $2, $3, $4, $5, $6)
		returning id, task_id, user_id, file_name, mime_type, size_bytes, storage_path, created_at
	`

	var attachment tasks.Attachment
	err := s.pool.QueryRow(
		ctx,
		query,
		params.TaskID,
		params.UserID,
		params.FileName,
		params.MimeType,
		params.SizeBytes,
		params.StoragePath,
	).Scan(
		&attachment.ID,
		&attachment.TaskID,
		&attachment.UserID,
		&attachment.FileName,
		&attachment.MimeType,
		&attachment.SizeBytes,
		&attachment.StoragePath,
		&attachment.CreatedAt,
	)
	if err != nil {
		return tasks.Attachment{}, fmt.Errorf("insert attachment: %w", err)
	}

	return attachment, nil
}

// ListAttachments returns attachments for a task if the viewer can access it.
func (s *PostgresStore) ListAttachments(ctx context.Context, taskID, userID string, isAdmin bool) ([]tasks.Attachment, error) {
	query := `
		select a.id, a.task_id, a.user_id, a.file_name, a.mime_type, a.size_bytes, a.storage_path, a.created_at
		from task_attachments a
		join tasks t on t.id = a.task_id
		where a.task_id = $1
	`
	args := []any{taskID}
	if !isAdmin {
		query += " and t.user_id = $2"
		args = append(args, userID)
	}
	query += " order by a.created_at desc"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list attachments: %w", err)
	}
	defer rows.Close()

	items := make([]tasks.Attachment, 0)
	for rows.Next() {
		var attachment tasks.Attachment
		if err := rows.Scan(
			&attachment.ID,
			&attachment.TaskID,
			&attachment.UserID,
			&attachment.FileName,
			&attachment.MimeType,
			&attachment.SizeBytes,
			&attachment.StoragePath,
			&attachment.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan attachment: %w", err)
		}
		items = append(items, attachment)
	}

	return items, nil
}

// GetAttachment fetches attachment metadata with ownership check.
func (s *PostgresStore) GetAttachment(ctx context.Context, attachmentID, userID string, isAdmin bool) (tasks.Attachment, error) {
	query := `
		select a.id, a.task_id, a.user_id, a.file_name, a.mime_type, a.size_bytes, a.storage_path, a.created_at
		from task_attachments a
		join tasks t on t.id = a.task_id
		where a.id = $1
	`
	args := []any{attachmentID}
	if !isAdmin {
		query += " and t.user_id = $2"
		args = append(args, userID)
	}

	var attachment tasks.Attachment
	err := s.pool.QueryRow(ctx, query, args...).Scan(
		&attachment.ID,
		&attachment.TaskID,
		&attachment.UserID,
		&attachment.FileName,
		&attachment.MimeType,
		&attachment.SizeBytes,
		&attachment.StoragePath,
		&attachment.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tasks.Attachment{}, tasks.ErrNotFound
		}
		return tasks.Attachment{}, fmt.Errorf("get attachment: %w", err)
	}

	return attachment, nil
}

// DeleteAttachment removes attachment metadata.
func (s *PostgresStore) DeleteAttachment(ctx context.Context, attachmentID, userID string, isAdmin bool) (tasks.Attachment, error) {
	query := `
		delete from task_attachments a
		using tasks t
		where a.task_id = t.id and a.id = $1
	`
	args := []any{attachmentID}
	if !isAdmin {
		query += " and t.user_id = $2"
		args = append(args, userID)
	}
	query += `
		returning a.id, a.task_id, a.user_id, a.file_name, a.mime_type, a.size_bytes, a.storage_path, a.created_at
	`

	var attachment tasks.Attachment
	err := s.pool.QueryRow(ctx, query, args...).Scan(
		&attachment.ID,
		&attachment.TaskID,
		&attachment.UserID,
		&attachment.FileName,
		&attachment.MimeType,
		&attachment.SizeBytes,
		&attachment.StoragePath,
		&attachment.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tasks.Attachment{}, tasks.ErrNotFound
		}
		return tasks.Attachment{}, fmt.Errorf("delete attachment: %w", err)
	}

	return attachment, nil
}
