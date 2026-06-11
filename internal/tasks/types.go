package tasks

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	StatusTodo       = "todo"
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
)

const (
	PriorityLow    = "low"
	PriorityMedium = "medium"
	PriorityHigh   = "high"
)

// Task represents a persisted task.
type Task struct {
	ID          uuid.UUID `json:"id"`
	UserID      string    `json:"user_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Priority    string    `json:"priority"`
	DueDate     time.Time `json:"due_date"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreateTaskInput is the input contract for task creation.
type CreateTaskInput struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Priority    string `json:"priority"`
	DueDate     string `json:"due_date"`
}

// UpdateTaskInput is the input contract for task updates.
type UpdateTaskInput struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
	Priority    *string `json:"priority"`
	DueDate     *string `json:"due_date"`
}

// ListParams controls filtering, search, sorting, and pagination.
type ListParams struct {
	Status   string
	Search   string
	SortBy   string
	SortDir  string
	Page     int
	PageSize int
}

// ListResult is the paginated task list response.
type ListResult struct {
	Items      []Task `json:"items"`
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
	Total      int    `json:"total"`
	TotalPages int    `json:"total_pages"`
}

// CreateTaskParams is the validated storage input for task creation.
type CreateTaskParams struct {
	UserID      string
	Title       string
	Description string
	Status      string
	Priority    string
	DueDate     time.Time
}

// UpdateTaskParams is the validated storage input for task updates.
type UpdateTaskParams struct {
	ID          string
	UserID      string
	IsAdmin     bool
	Title       *string
	Description *string
	Status      *string
	Priority    *string
	DueDate     *time.Time
}

// Store defines task persistence operations.
type Store interface {
	CreateTask(ctx context.Context, params CreateTaskParams) (Task, error)
	ListTasks(ctx context.Context, userID string, isAdmin bool, params ListParams) (ListResult, error)
	GetTask(ctx context.Context, id string, userID string, isAdmin bool) (Task, error)
	UpdateTask(ctx context.Context, params UpdateTaskParams) (Task, error)
	DeleteTask(ctx context.Context, id string, userID string, isAdmin bool) error
}

// ErrNotFound is returned when a task cannot be found.
var ErrNotFound = fmt.Errorf("task not found")

// ValidateTaskID ensures route task IDs are valid UUIDs.
func ValidateTaskID(id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("task id is required")
	}
	if _, err := uuid.Parse(id); err != nil {
		return fmt.Errorf("task id must be a valid UUID")
	}
	return nil
}

// ParseListParams validates list query params and applies defaults.
func ParseListParams(values url.Values) (ListParams, error) {
	params := ListParams{
		Status:   strings.TrimSpace(values.Get("status")),
		Search:   strings.TrimSpace(values.Get("search")),
		SortBy:   values.Get("sort_by"),
		SortDir:  values.Get("sort_dir"),
		Page:     1,
		PageSize: 10,
	}

	if params.Status != "" && !isAllowedStatus(params.Status) {
		return ListParams{}, fmt.Errorf("status must be one of todo, in_progress, completed")
	}

	if page := values.Get("page"); page != "" {
		parsed, err := strconv.Atoi(page)
		if err != nil || parsed < 1 {
			return ListParams{}, fmt.Errorf("page must be a positive integer")
		}
		params.Page = parsed
	}

	if pageSize := values.Get("page_size"); pageSize != "" {
		parsed, err := strconv.Atoi(pageSize)
		if err != nil || parsed < 1 || parsed > 50 {
			return ListParams{}, fmt.Errorf("page_size must be between 1 and 50")
		}
		params.PageSize = parsed
	}

	if params.SortBy == "" {
		params.SortBy = "created_at"
	}
	if params.SortDir == "" {
		params.SortDir = "desc"
	}

	if !isAllowedSortBy(params.SortBy) {
		return ListParams{}, fmt.Errorf("sort_by must be one of created_at, due_date, priority")
	}
	if params.SortDir != "asc" && params.SortDir != "desc" {
		return ListParams{}, fmt.Errorf("sort_dir must be asc or desc")
	}

	return params, nil
}

// ValidateCreate validates and normalizes task creation input.
func ValidateCreate(input CreateTaskInput) (CreateTaskParams, error) {
	if strings.TrimSpace(input.Title) == "" {
		return CreateTaskParams{}, fmt.Errorf("title is required")
	}
	if !isAllowedStatus(input.Status) {
		return CreateTaskParams{}, fmt.Errorf("status must be one of todo, in_progress, completed")
	}
	if !isAllowedPriority(input.Priority) {
		return CreateTaskParams{}, fmt.Errorf("priority must be one of low, medium, high")
	}
	if strings.TrimSpace(input.DueDate) == "" {
		return CreateTaskParams{}, fmt.Errorf("due_date is required")
	}

	dueDate, err := time.Parse(time.RFC3339, input.DueDate)
	if err != nil {
		return CreateTaskParams{}, fmt.Errorf("due_date must be a valid RFC3339 timestamp")
	}

	return CreateTaskParams{
		Title:       strings.TrimSpace(input.Title),
		Description: strings.TrimSpace(input.Description),
		Status:      input.Status,
		Priority:    input.Priority,
		DueDate:     dueDate.UTC(),
	}, nil
}

// ValidateUpdate validates and normalizes task update input.
func ValidateUpdate(input UpdateTaskInput) (UpdateTaskParams, error) {
	if input.Title == nil && input.Description == nil && input.Status == nil && input.Priority == nil && input.DueDate == nil {
		return UpdateTaskParams{}, fmt.Errorf("at least one field is required")
	}

	params := UpdateTaskParams{
		Title:       input.Title,
		Description: input.Description,
		Status:      input.Status,
		Priority:    input.Priority,
	}

	if input.Title != nil && strings.TrimSpace(*input.Title) == "" {
		return UpdateTaskParams{}, fmt.Errorf("title cannot be empty")
	}
	if input.Status != nil && !isAllowedStatus(*input.Status) {
		return UpdateTaskParams{}, fmt.Errorf("status must be one of todo, in_progress, completed")
	}
	if input.Priority != nil && !isAllowedPriority(*input.Priority) {
		return UpdateTaskParams{}, fmt.Errorf("priority must be one of low, medium, high")
	}
	if input.DueDate != nil {
		dueDate, err := time.Parse(time.RFC3339, *input.DueDate)
		if err != nil {
			return UpdateTaskParams{}, fmt.Errorf("due_date must be a valid RFC3339 timestamp")
		}
		params.DueDate = &dueDate
	}
	if input.Title != nil {
		title := strings.TrimSpace(*input.Title)
		params.Title = &title
	}
	if input.Description != nil {
		description := strings.TrimSpace(*input.Description)
		params.Description = &description
	}

	return params, nil
}

func isAllowedStatus(value string) bool {
	switch value {
	case StatusTodo, StatusInProgress, StatusCompleted:
		return true
	default:
		return false
	}
}

func isAllowedPriority(value string) bool {
	switch value {
	case PriorityLow, PriorityMedium, PriorityHigh:
		return true
	default:
		return false
	}
}

func isAllowedSortBy(value string) bool {
	switch value {
	case "created_at", "due_date", "priority":
		return true
	default:
		return false
	}
}
