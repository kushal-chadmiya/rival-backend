package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"taskboard-backend/internal/auth"
)

// NewRouter builds the HTTP router.
func NewRouter(deps RouterDependencies) http.Handler {
	router := chi.NewRouter()

	router.Use(corsMiddleware(deps.FrontendURL))

	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	router.Route("/auth", func(r chi.Router) {
		r.Post("/signup", signupHandler(deps.AuthClient))
		r.Post("/login", loginHandler(deps.AuthClient))
		r.Post("/refresh", refreshHandler(deps.AuthClient))
		r.Group(func(r chi.Router) {
			r.Use(auth.AuthMiddleware(deps.Verifier, deps.AllowViewAsAdmin))
			r.Get("/me", meHandler(deps.AllowViewAsAdmin))
		})
	})

	router.Get("/tasks/events", taskEventsHandler(deps))

	router.Route("/tasks", func(r chi.Router) {
		r.Use(auth.AuthMiddleware(deps.Verifier, deps.AllowViewAsAdmin))
		r.Get("/", listTasksHandler(deps))
		r.Post("/", createTaskHandler(deps))
		r.Get("/{taskID}/activity", listTaskActivityHandler(deps))
		r.Get("/{taskID}/attachments", listAttachmentsHandler(deps))
		r.Post("/{taskID}/attachments", uploadAttachmentHandler(deps))
		r.Delete("/{taskID}/attachments/{attachmentID}", deleteAttachmentHandler(deps))
		r.Get("/{taskID}", getTaskHandler(deps))
		r.Patch("/{taskID}", updateTaskHandler(deps))
		r.Delete("/{taskID}", deleteTaskHandler(deps))
	})

	return router
}
