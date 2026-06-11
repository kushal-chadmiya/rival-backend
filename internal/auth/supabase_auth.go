package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client wraps the Supabase auth REST API for signup and login.
type Client struct {
	baseURL    string
	anonKey    string
	httpClient *http.Client
}

// NewClient builds a Supabase auth client.
func NewClient(baseURL string, anonKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		anonKey: anonKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SessionResponse mirrors the subset of the Supabase auth response used by the frontend.
type SessionResponse struct {
	AccessToken  string           `json:"access_token"`
	RefreshToken string           `json:"refresh_token"`
	ExpiresIn    int              `json:"expires_in"`
	TokenType    string           `json:"token_type"`
	User         sessionUser      `json:"user"`
	Session      *SessionResponse `json:"session,omitempty"`
}

type sessionUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// Credentials contains login/signup input data.
type Credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Signup creates a Supabase user.
func (c *Client) Signup(ctx context.Context, credentials Credentials) (SessionResponse, error) {
	return c.post(ctx, "/auth/v1/signup", credentials)
}

// Login signs a Supabase user in using password grant.
func (c *Client) Login(ctx context.Context, credentials Credentials) (SessionResponse, error) {
	return c.post(ctx, "/auth/v1/token?grant_type=password", credentials)
}

// Refresh exchanges a refresh token for a new access token.
func (c *Client) Refresh(ctx context.Context, refreshToken string) (SessionResponse, error) {
	return c.post(ctx, "/auth/v1/token?grant_type=refresh_token", map[string]string{
		"refresh_token": refreshToken,
	})
}

// HasAccessToken reports whether the auth response includes an active session.
func (s SessionResponse) HasAccessToken() bool {
	return strings.TrimSpace(s.AccessToken) != ""
}

func (c *Client) post(ctx context.Context, path string, payload any) (SessionResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return SessionResponse{}, fmt.Errorf("marshal auth request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return SessionResponse{}, fmt.Errorf("build auth request: %w", err)
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("apikey", c.anonKey)
	request.Header.Set("Authorization", "Bearer "+c.anonKey)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return SessionResponse{}, fmt.Errorf("execute auth request: %w", err)
	}
	defer response.Body.Close()

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return SessionResponse{}, fmt.Errorf("read auth response: %w", err)
	}

	if response.StatusCode >= http.StatusBadRequest {
		var parsed struct {
			Message string `json:"msg"`
			Error   string `json:"error_description"`
		}

		_ = json.Unmarshal(data, &parsed)

		message := parsed.Error
		if message == "" {
			message = parsed.Message
		}
		if message == "" {
			message = "authentication request failed"
		}

		return SessionResponse{}, fmt.Errorf("%s", message)
	}

	var parsed SessionResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		return SessionResponse{}, fmt.Errorf("decode auth response: %w", err)
	}

	if parsed.Session != nil {
		return *parsed.Session, nil
	}

	return parsed, nil
}
