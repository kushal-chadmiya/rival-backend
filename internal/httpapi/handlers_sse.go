package httpapi

import (
	"net/http"

	"taskboard-backend/internal/auth"
	"taskboard-backend/internal/realtime"
)

func taskEventsHandler(deps RouterDependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := realtime.TokenFromQuery(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized", "access_token is required")
			return
		}

		viewer, err := deps.Verifier.Verify(r.Context(), token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired access token")
			return
		}

		viewer = auth.WithViewRole(viewer, r.URL.Query().Get("view_role"), deps.AllowViewAsAdmin)
		realtime.ServeSSE(w, r, deps.Hub, viewer.UserID, viewer.IsAdmin())
	}
}
