package httpapi

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"

	"taskboard-backend/internal/activity"
	"taskboard-backend/internal/auth"
	"taskboard-backend/internal/realtime"
	"taskboard-backend/internal/tasks"
)

func listTasksHandler(deps RouterDependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		viewer, ok := auth.ViewerFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "viewer is required")
			return
		}

		params, err := tasks.ParseListParams(r.URL.Query())
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
			return
		}

		result, err := deps.TaskStore.ListTasks(r.Context(), viewer.UserID, viewer.IsAdmin(), params)
		if err != nil {
			log.Printf("list tasks: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to load tasks")
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

func createTaskHandler(deps RouterDependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		viewer, ok := auth.ViewerFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "viewer is required")
			return
		}

		var input tasks.CreateTaskInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
			return
		}

		params, err := tasks.ValidateCreate(input)
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
			return
		}
		params.UserID = viewer.UserID

		task, err := deps.TaskStore.CreateTask(r.Context(), params)
		if err != nil {
			log.Printf("create task: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to create task")
			return
		}

		if deps.ActivityStore != nil {
			_ = deps.ActivityStore.Record(r.Context(), task.ID.String(), viewer, activity.ActionCreated, map[string]any{
				"title":    task.Title,
				"status":   task.Status,
				"priority": task.Priority,
				"due_date": task.DueDate,
			})
		}
		broadcastTask(deps, realtime.EventTaskCreated, task)

		writeJSON(w, http.StatusCreated, task)
	}
}

func getTaskHandler(deps RouterDependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		viewer, ok := auth.ViewerFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "viewer is required")
			return
		}

		taskID := chi.URLParam(r, "taskID")
		if err := tasks.ValidateTaskID(taskID); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
			return
		}

		task, err := deps.TaskStore.GetTask(r.Context(), taskID, viewer.UserID, viewer.IsAdmin())
		if err != nil {
			writeTaskStoreError(w, err, "failed to load task", "task not found")
			return
		}

		writeJSON(w, http.StatusOK, task)
	}
}

func updateTaskHandler(deps RouterDependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		viewer, ok := auth.ViewerFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "viewer is required")
			return
		}

		taskID := chi.URLParam(r, "taskID")
		if err := tasks.ValidateTaskID(taskID); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
			return
		}

		before, err := deps.TaskStore.GetTask(r.Context(), taskID, viewer.UserID, viewer.IsAdmin())
		if err != nil {
			writeTaskStoreError(w, err, "failed to load task", "task not found")
			return
		}
		if !writeUnlessTaskOwner(w, before.UserID, viewer.UserID) {
			return
		}

		var input tasks.UpdateTaskInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
			return
		}

		params, err := tasks.ValidateUpdate(input)
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
			return
		}
		params.ID = taskID
		params.UserID = viewer.UserID
		params.IsAdmin = false

		task, err := deps.TaskStore.UpdateTask(r.Context(), params)
		if err != nil {
			writeTaskStoreError(w, err, "failed to update task", "task not found")
			return
		}

		if deps.ActivityStore != nil {
			_ = deps.ActivityStore.Record(r.Context(), task.ID.String(), viewer, activity.ActionUpdated, taskDiff(before, task))
		}
		broadcastTask(deps, realtime.EventTaskUpdated, task)

		writeJSON(w, http.StatusOK, task)
	}
}

func deleteTaskHandler(deps RouterDependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		viewer, ok := auth.ViewerFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "viewer is required")
			return
		}

		taskID := chi.URLParam(r, "taskID")
		if err := tasks.ValidateTaskID(taskID); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
			return
		}

		before, err := deps.TaskStore.GetTask(r.Context(), taskID, viewer.UserID, viewer.IsAdmin())
		if err != nil {
			writeTaskStoreError(w, err, "failed to load task", "task not found")
			return
		}
		if !writeUnlessTaskOwner(w, before.UserID, viewer.UserID) {
			return
		}

		if err := deps.TaskStore.DeleteTask(r.Context(), taskID, viewer.UserID, false); err != nil {
			writeTaskStoreError(w, err, "failed to delete task", "task not found")
			return
		}

		if deps.ActivityStore != nil {
			_ = deps.ActivityStore.Record(r.Context(), taskID, viewer, activity.ActionDeleted, map[string]any{
				"title": before.Title,
			})
		}
		if deps.Hub != nil {
			deps.Hub.Broadcast(realtime.Event{
				Type:   realtime.EventTaskDeleted,
				TaskID: taskID,
			})
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func broadcastTask(deps RouterDependencies, eventType realtime.EventType, task tasks.Task) {
	if deps.Hub == nil {
		return
	}
	copied := task
	deps.Hub.Broadcast(realtime.Event{
		Type:   eventType,
		Task:   &copied,
		TaskID: task.ID.String(),
	})
}

func writeUnlessTaskOwner(w http.ResponseWriter, ownerID, viewerID string) bool {
	if ownerID == viewerID {
		return true
	}

	writeError(w, http.StatusForbidden, "forbidden", "you can only modify your own tasks")
	return false
}

func taskDiff(before, after tasks.Task) map[string]any {
	changes := map[string]any{}
	if before.Title != after.Title {
		changes["title"] = map[string]string{"from": before.Title, "to": after.Title}
	}
	if before.Description != after.Description {
		changes["description"] = map[string]string{"from": before.Description, "to": after.Description}
	}
	if before.Status != after.Status {
		changes["status"] = map[string]string{"from": before.Status, "to": after.Status}
	}
	if before.Priority != after.Priority {
		changes["priority"] = map[string]string{"from": before.Priority, "to": after.Priority}
	}
	if !before.DueDate.Equal(after.DueDate) {
		changes["due_date"] = map[string]string{"from": before.DueDate.Format("2006-01-02T15:04:05Z07:00"), "to": after.DueDate.Format("2006-01-02T15:04:05Z07:00")}
	}
	return changes
}
