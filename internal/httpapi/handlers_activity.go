package httpapi

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"

	"taskboard-backend/internal/auth"
	"taskboard-backend/internal/tasks"
)

func listTaskActivityHandler(deps RouterDependencies) http.HandlerFunc {
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

		if _, err := deps.TaskStore.GetTask(r.Context(), taskID, viewer.UserID, viewer.IsAdmin()); err != nil {
			status := http.StatusInternalServerError
			code := "internal_error"
			message := "failed to load task"
			if err == tasks.ErrNotFound {
				status = http.StatusNotFound
				code = "not_found"
				message = "task not found"
			}
			writeError(w, status, code, message)
			return
		}

		entries, err := deps.ActivityStore.ListByTask(r.Context(), taskID, viewer.UserID, viewer.IsAdmin())
		if err != nil {
			log.Printf("list activity: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to load activity")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"items": entries})
	}
}
