package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"taskboard-backend/internal/auth"
	"taskboard-backend/internal/realtime"
	"taskboard-backend/internal/tasks"
)

type stubVerifier struct{}

func (stubVerifier) Verify(context.Context, string) (auth.Viewer, error) {
	return auth.Viewer{
		UserID:     "user-123",
		Email:      "demo@example.com",
		ActualRole: "authenticated",
		Role:       "authenticated",
	}, nil
}

func (stubVerifier) Close(context.Context) error {
	return nil
}

type stubStore struct {
	createFn func(ctx context.Context, params tasks.CreateTaskParams) (tasks.Task, error)
	listFn   func(ctx context.Context, userID string, isAdmin bool, params tasks.ListParams) (tasks.ListResult, error)
	getFn    func(ctx context.Context, id string, userID string, isAdmin bool) (tasks.Task, error)
	updateFn func(ctx context.Context, params tasks.UpdateTaskParams) (tasks.Task, error)
	deleteFn func(ctx context.Context, id string, userID string, isAdmin bool) error
}

func (s stubStore) CreateTask(ctx context.Context, params tasks.CreateTaskParams) (tasks.Task, error) {
	return s.createFn(ctx, params)
}

func (s stubStore) ListTasks(ctx context.Context, userID string, isAdmin bool, params tasks.ListParams) (tasks.ListResult, error) {
	return s.listFn(ctx, userID, isAdmin, params)
}

func (s stubStore) GetTask(ctx context.Context, id string, userID string, isAdmin bool) (tasks.Task, error) {
	if s.getFn != nil {
		return s.getFn(ctx, id, userID, isAdmin)
	}
	return tasks.Task{}, tasks.ErrNotFound
}

func (s stubStore) UpdateTask(ctx context.Context, params tasks.UpdateTaskParams) (tasks.Task, error) {
	if s.updateFn != nil {
		return s.updateFn(ctx, params)
	}
	return tasks.Task{}, tasks.ErrNotFound
}

func (s stubStore) DeleteTask(ctx context.Context, id string, userID string, isAdmin bool) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, id, userID, isAdmin)
	}
	return tasks.ErrNotFound
}

func (stubStore) AddAttachment(context.Context, tasks.CreateAttachmentParams) (tasks.Attachment, error) {
	return tasks.Attachment{}, nil
}

func (stubStore) ListAttachments(context.Context, string, string, bool) ([]tasks.Attachment, error) {
	return []tasks.Attachment{}, nil
}

func (stubStore) GetAttachment(context.Context, string, string, bool) (tasks.Attachment, error) {
	return tasks.Attachment{}, tasks.ErrNotFound
}

func (stubStore) DeleteAttachment(context.Context, string, string, bool) (tasks.Attachment, error) {
	return tasks.Attachment{}, tasks.ErrNotFound
}

type adminVerifier struct{}

func (adminVerifier) Verify(context.Context, string) (auth.Viewer, error) {
	return auth.Viewer{
		UserID:     "admin-1",
		Email:      "admin@example.com",
		ActualRole: "admin",
		Role:       "admin",
	}, nil
}

func (adminVerifier) Close(context.Context) error { return nil }

func testRouter(store stubStore) http.Handler {
	return testRouterWithVerifier(store, stubVerifier{})
}

func testRouterWithVerifier(store stubStore, verifier auth.Verifier) http.Handler {
	return NewRouter(RouterDependencies{
		FrontendURL:      "http://localhost:3000",
		Verifier:         verifier,
		AuthClient:       auth.NewClient("https://example.supabase.co", "anon"),
		TaskStore:        store,
		Hub:              realtime.NewHub(),
		AllowViewAsAdmin: true,
	})
}

func defaultStubStore() stubStore {
	return stubStore{
		createFn: func(context.Context, tasks.CreateTaskParams) (tasks.Task, error) {
			return tasks.Task{}, nil
		},
		listFn: func(context.Context, string, bool, tasks.ListParams) (tasks.ListResult, error) {
			return tasks.ListResult{}, nil
		},
	}
}

func stubStoreWithTask() stubStore {
	store := defaultStubStore()
	store.getFn = func(_ context.Context, id string, userID string, _ bool) (tasks.Task, error) {
		return tasks.Task{
			ID:     uuid.MustParse(id),
			UserID: userID,
			Title:  "Existing task",
			Status: "todo",
		}, nil
	}
	return store
}

func authRequest(method, target string, body []byte) *http.Request {
	var request *http.Request
	if body == nil {
		request = httptest.NewRequest(method, target, nil)
	} else {
		request = httptest.NewRequest(method, target, bytes.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set("Authorization", "Bearer test-token")
	return request
}

func TestCreateTaskHandlerReturnsCreatedTask(t *testing.T) {
	t.Parallel()

	router := testRouter(stubStore{
		createFn: func(ctx context.Context, params tasks.CreateTaskParams) (tasks.Task, error) {
			return tasks.Task{
				ID:          uuid.MustParse("8dba5f0e-f34f-4c69-b4d4-94bf304bc4de"),
				UserID:      params.UserID,
				Title:       params.Title,
				Description: params.Description,
				Status:      params.Status,
				Priority:    params.Priority,
				DueDate:     params.DueDate,
				CreatedAt:   time.Now().UTC(),
				UpdatedAt:   time.Now().UTC(),
			}, nil
		},
		listFn: func(context.Context, string, bool, tasks.ListParams) (tasks.ListResult, error) {
			return tasks.ListResult{}, nil
		},
	})

	body, _ := json.Marshal(map[string]string{
		"title":       "Ship backend",
		"description": "Finish CRUD routes",
		"status":      "todo",
		"priority":    "high",
		"due_date":    "2026-06-30T10:00:00Z",
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authRequest(http.MethodPost, "/tasks", body))

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", recorder.Code)
	}
}

func TestCreateTaskHandlerRejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	router := testRouter(defaultStubStore())
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authRequest(http.MethodPost, "/tasks", []byte("{")))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
	assertErrorCode(t, recorder, "invalid_json")
}

func TestCreateTaskHandlerRejectsValidationError(t *testing.T) {
	t.Parallel()

	router := testRouter(defaultStubStore())
	body, _ := json.Marshal(map[string]string{
		"title":    "",
		"status":   "todo",
		"priority": "high",
		"due_date": "2026-06-30T10:00:00Z",
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authRequest(http.MethodPost, "/tasks", body))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
	assertErrorCode(t, recorder, "validation_error")
}

func TestListTasksHandlerReturnsOK(t *testing.T) {
	t.Parallel()

	router := testRouter(stubStore{
		createFn: func(context.Context, tasks.CreateTaskParams) (tasks.Task, error) {
			return tasks.Task{}, nil
		},
		listFn: func(ctx context.Context, userID string, isAdmin bool, params tasks.ListParams) (tasks.ListResult, error) {
			if params.Status != "todo" || params.Page != 1 || params.PageSize != 10 {
				t.Fatalf("unexpected list params: %#v", params)
			}
			return tasks.ListResult{
				Items:      []tasks.Task{},
				Page:       params.Page,
				PageSize:   params.PageSize,
				Total:      0,
				TotalPages: 0,
			}, nil
		},
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authRequest(http.MethodGet, "/tasks?status=todo&page=1&page_size=10", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestGetTaskHandlerReturnsTask(t *testing.T) {
	t.Parallel()

	taskID := "8dba5f0e-f34f-4c69-b4d4-94bf304bc4de"
	router := testRouter(stubStore{
		createFn: func(context.Context, tasks.CreateTaskParams) (tasks.Task, error) { return tasks.Task{}, nil },
		listFn:   func(context.Context, string, bool, tasks.ListParams) (tasks.ListResult, error) { return tasks.ListResult{}, nil },
		getFn: func(_ context.Context, id string, userID string, _ bool) (tasks.Task, error) {
			return tasks.Task{
				ID:     uuid.MustParse(id),
				UserID: userID,
				Title:  "Ship backend",
				Status: "todo",
			}, nil
		},
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authRequest(http.MethodGet, "/tasks/"+taskID, nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestGetTaskHandlerReturnsNotFound(t *testing.T) {
	t.Parallel()

	router := testRouter(defaultStubStore())
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authRequest(http.MethodGet, "/tasks/8dba5f0e-f34f-4c69-b4d4-94bf304bc4de", nil))

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", recorder.Code)
	}
	assertErrorCode(t, recorder, "not_found")
}

func TestGetTaskHandlerRejectsInvalidID(t *testing.T) {
	t.Parallel()

	router := testRouter(defaultStubStore())
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authRequest(http.MethodGet, "/tasks/not-a-uuid", nil))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
	assertErrorCode(t, recorder, "validation_error")
}

func TestUpdateTaskHandlerReturnsUpdatedTask(t *testing.T) {
	t.Parallel()

	taskID := "8dba5f0e-f34f-4c69-b4d4-94bf304bc4de"
	router := testRouter(stubStore{
		createFn: func(context.Context, tasks.CreateTaskParams) (tasks.Task, error) { return tasks.Task{}, nil },
		listFn:   func(context.Context, string, bool, tasks.ListParams) (tasks.ListResult, error) { return tasks.ListResult{}, nil },
		getFn: func(_ context.Context, id string, userID string, _ bool) (tasks.Task, error) {
			return tasks.Task{ID: uuid.MustParse(id), UserID: userID, Title: "Before", Status: "todo"}, nil
		},
		updateFn: func(_ context.Context, params tasks.UpdateTaskParams) (tasks.Task, error) {
			title := *params.Title
			return tasks.Task{
				ID:     uuid.MustParse(params.ID),
				UserID: params.UserID,
				Title:  title,
				Status: "in_progress",
			}, nil
		},
	})

	body, _ := json.Marshal(map[string]string{
		"title":  "Updated title",
		"status": "in_progress",
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authRequest(http.MethodPatch, "/tasks/"+taskID, body))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestUpdateTaskHandlerRejectsEmptyBody(t *testing.T) {
	t.Parallel()

	router := testRouter(stubStoreWithTask())
	body, _ := json.Marshal(map[string]string{})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authRequest(http.MethodPatch, "/tasks/8dba5f0e-f34f-4c69-b4d4-94bf304bc4de", body))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
	assertErrorCode(t, recorder, "validation_error")
}

func TestDeleteTaskHandlerReturnsNoContent(t *testing.T) {
	t.Parallel()

	taskID := "8dba5f0e-f34f-4c69-b4d4-94bf304bc4de"
	router := testRouter(stubStore{
		createFn: func(context.Context, tasks.CreateTaskParams) (tasks.Task, error) { return tasks.Task{}, nil },
		listFn:   func(context.Context, string, bool, tasks.ListParams) (tasks.ListResult, error) { return tasks.ListResult{}, nil },
		getFn: func(_ context.Context, id string, userID string, _ bool) (tasks.Task, error) {
			return tasks.Task{ID: uuid.MustParse(id), UserID: userID, Title: "Delete me"}, nil
		},
		deleteFn: func(_ context.Context, id string, _ string, _ bool) error {
			if id != taskID {
				t.Fatalf("unexpected delete id: %s", id)
			}
			return nil
		},
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authRequest(http.MethodDelete, "/tasks/"+taskID, nil))

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", recorder.Code)
	}
}

func TestDeleteTaskHandlerReturnsNotFound(t *testing.T) {
	t.Parallel()

	router := testRouter(defaultStubStore())
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authRequest(http.MethodDelete, "/tasks/8dba5f0e-f34f-4c69-b4d4-94bf304bc4de", nil))

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", recorder.Code)
	}
	assertErrorCode(t, recorder, "not_found")
}

func TestLoginHandlerRejectsInvalidCredentials(t *testing.T) {
	t.Parallel()

	router := testRouter(defaultStubStore())
	body, _ := json.Marshal(map[string]string{
		"email":    "demo@example.com",
		"password": "short",
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authRequest(http.MethodPost, "/auth/login", body))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}

func TestRefreshHandlerRequiresToken(t *testing.T) {
	t.Parallel()

	router := testRouter(defaultStubStore())
	body, _ := json.Marshal(map[string]string{})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authRequest(http.MethodPost, "/auth/refresh", body))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
	assertErrorCode(t, recorder, "validation_error")
}

func TestMeHandlerReturnsProfile(t *testing.T) {
	t.Parallel()

	router := testRouter(defaultStubStore())
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authRequest(http.MethodGet, "/auth/me", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestAdminListTasksUsesUserViewWhenRequested(t *testing.T) {
	t.Parallel()

	router := testRouterWithVerifier(stubStore{
		createFn: func(context.Context, tasks.CreateTaskParams) (tasks.Task, error) { return tasks.Task{}, nil },
		listFn: func(_ context.Context, _ string, isAdmin bool, _ tasks.ListParams) (tasks.ListResult, error) {
			if isAdmin {
				t.Fatal("expected user view list call")
			}
			return tasks.ListResult{}, nil
		},
	}, adminVerifier{})

	recorder := httptest.NewRecorder()
	request := authRequest(http.MethodGet, "/tasks", nil)
	request.Header.Set(auth.ViewRoleHeader, "user")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestAdminListTasksUsesAdminFlag(t *testing.T) {
	t.Parallel()

	router := testRouterWithVerifier(stubStore{
		createFn: func(context.Context, tasks.CreateTaskParams) (tasks.Task, error) { return tasks.Task{}, nil },
		listFn: func(_ context.Context, _ string, isAdmin bool, _ tasks.ListParams) (tasks.ListResult, error) {
			if !isAdmin {
				t.Fatal("expected admin list call")
			}
			return tasks.ListResult{Items: []tasks.Task{{UserID: "other-user"}}}, nil
		},
	}, adminVerifier{})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authRequest(http.MethodGet, "/tasks", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestTasksRequireAuthorization(t *testing.T) {
	t.Parallel()

	router := testRouter(defaultStubStore())
	request := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", recorder.Code)
	}
	assertErrorCode(t, recorder, "unauthorized")
}

func assertErrorCode(t *testing.T, recorder *httptest.ResponseRecorder, code string) {
	t.Helper()

	var payload struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if payload.Error.Code != code {
		t.Fatalf("expected error code %q, got %q", code, payload.Error.Code)
	}
	if payload.Error.Message == "" {
		t.Fatal("expected non-empty error message")
	}
}
