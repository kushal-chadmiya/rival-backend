package httpapi

import (
	"context"

	"taskboard-backend/internal/activity"
	"taskboard-backend/internal/auth"
	"taskboard-backend/internal/realtime"
	"taskboard-backend/internal/storage"
	"taskboard-backend/internal/tasks"
)

// TaskStore is the task persistence contract used by handlers.
type TaskStore interface {
	CreateTask(ctx context.Context, params tasks.CreateTaskParams) (tasks.Task, error)
	ListTasks(ctx context.Context, userID string, isAdmin bool, params tasks.ListParams) (tasks.ListResult, error)
	GetTask(ctx context.Context, id string, userID string, isAdmin bool) (tasks.Task, error)
	UpdateTask(ctx context.Context, params tasks.UpdateTaskParams) (tasks.Task, error)
	DeleteTask(ctx context.Context, id string, userID string, isAdmin bool) error
	AddAttachment(ctx context.Context, params tasks.CreateAttachmentParams) (tasks.Attachment, error)
	ListAttachments(ctx context.Context, taskID, userID string, isAdmin bool) ([]tasks.Attachment, error)
	GetAttachment(ctx context.Context, attachmentID, userID string, isAdmin bool) (tasks.Attachment, error)
	DeleteAttachment(ctx context.Context, attachmentID, userID string, isAdmin bool) (tasks.Attachment, error)
}

// RouterDependencies holds all handler dependencies.
type RouterDependencies struct {
	FrontendURL    string
	Verifier       auth.Verifier
	AuthClient     *auth.Client
	TaskStore      TaskStore
	ActivityStore  *activity.Store
	FileStorage    *storage.Client
	Hub                *realtime.Hub
	MaxUploadBytes     int64
	AllowViewAsAdmin   bool
}
