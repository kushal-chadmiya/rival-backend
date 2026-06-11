package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"taskboard-backend/internal/auth"
)

func signupHandler(client *auth.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var credentials auth.Credentials
		if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
			return
		}
		if strings.TrimSpace(credentials.Email) == "" || len(credentials.Password) < 8 {
			writeError(w, http.StatusBadRequest, "validation_error", "email and password (min 8 chars) are required")
			return
		}

		session, err := client.Signup(r.Context(), credentials)
		if err != nil {
			writeError(w, http.StatusBadRequest, "signup_failed", err.Error())
			return
		}
		if !session.HasAccessToken() {
			writeJSON(w, http.StatusAccepted, map[string]string{
				"message": "account created; check your email to confirm before signing in",
			})
			return
		}

		writeJSON(w, http.StatusCreated, session)
	}
}

func loginHandler(client *auth.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var credentials auth.Credentials
		if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
			return
		}
		if strings.TrimSpace(credentials.Email) == "" || len(credentials.Password) < 8 {
			writeError(w, http.StatusBadRequest, "validation_error", "email and password (min 8 chars) are required")
			return
		}

		session, err := client.Login(r.Context(), credentials)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "login_failed", err.Error())
			return
		}

		writeJSON(w, http.StatusOK, session)
	}
}

func refreshHandler(client *auth.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			RefreshToken string `json:"refresh_token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
			return
		}
		if strings.TrimSpace(input.RefreshToken) == "" {
			writeError(w, http.StatusBadRequest, "validation_error", "refresh_token is required")
			return
		}

		session, err := client.Refresh(r.Context(), input.RefreshToken)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "refresh_failed", err.Error())
			return
		}
		if !session.HasAccessToken() {
			writeError(w, http.StatusUnauthorized, "refresh_failed", "refresh token is invalid or expired")
			return
		}

		writeJSON(w, http.StatusOK, session)
	}
}

func meHandler(allowViewAsAdmin bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		viewer, ok := auth.ViewerFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "viewer is required")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"user_id":          viewer.UserID,
			"email":            viewer.Email,
			"role":             viewer.Role,
			"actual_role":      viewer.ActualRole,
			"is_admin":         viewer.IsAdmin(),
			"can_toggle_admin": viewer.HasAdminGrant() || allowViewAsAdmin,
		})
	}
}
