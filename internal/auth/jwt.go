package auth

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

// Verifier validates bearer tokens and extracts viewer claims.
type Verifier interface {
	Verify(ctx context.Context, token string) (Viewer, error)
	Close(context.Context) error
}

type jwtVerifier struct {
	parser *jwt.Parser
	keyset keySource
}

type keySource interface {
	Keyfunc(token *jwt.Token) (any, error)
	Close(context.Context) error
}

type hmacKeySource struct {
	secret []byte
}

type remoteKeySource struct {
	inner keyfunc.Keyfunc
}

func (h hmacKeySource) Keyfunc(token *jwt.Token) (any, error) {
	if token.Method.Alg() != jwt.SigningMethodHS256.Alg() && token.Method.Alg() != jwt.SigningMethodHS512.Alg() {
		return nil, fmt.Errorf("unexpected signing algorithm %s", token.Method.Alg())
	}

	return h.secret, nil
}

func (h hmacKeySource) Close(context.Context) error {
	return nil
}

func (r remoteKeySource) Close(context.Context) error {
	return nil
}

func (r remoteKeySource) Keyfunc(token *jwt.Token) (any, error) {
	return r.inner.Keyfunc(token)
}

type supabaseClaims struct {
	Email       string `json:"email"`
	Role        string `json:"role"`
	AppMetadata struct {
		Role string `json:"role"`
	} `json:"app_metadata"`
	jwt.RegisteredClaims
}

// NewVerifier creates a verifier backed by the Supabase JWT secret or JWKS.
func NewVerifier(ctx context.Context, jwtSecret string, jwksURL string) (Verifier, error) {
	var source keySource

	if jwtSecret != "" {
		source = hmacKeySource{secret: []byte(jwtSecret)}
	} else {
		jwks, err := keyfunc.NewDefaultCtx(ctx, []string{jwksURL})
		if err != nil {
			return nil, fmt.Errorf("create jwks verifier: %w", err)
		}
		source = remoteKeySource{inner: jwks}
	}

	return &jwtVerifier{
		parser: jwt.NewParser(
			jwt.WithValidMethods([]string{"RS256", "ES256", "HS256", "HS512"}),
			jwt.WithLeeway(30*time.Second),
		),
		keyset: source,
	}, nil
}

func (v *jwtVerifier) Verify(ctx context.Context, token string) (Viewer, error) {
	claims := &supabaseClaims{}

	parsedToken, err := v.parser.ParseWithClaims(token, claims, v.keyset.Keyfunc)
	if err != nil {
		return Viewer{}, fmt.Errorf("parse token: %w", err)
	}

	if !parsedToken.Valid {
		return Viewer{}, fmt.Errorf("invalid token")
	}

	if claims.Subject == "" {
		return Viewer{}, fmt.Errorf("token subject is required")
	}

	role := claims.Role
	if claims.AppMetadata.Role != "" {
		role = claims.AppMetadata.Role
	}

	return Viewer{
		UserID:     claims.Subject,
		Email:      claims.Email,
		ActualRole: role,
		Role:       role,
	}, nil
}

func (v *jwtVerifier) Close(ctx context.Context) error {
	return v.keyset.Close(ctx)
}

// AuthMiddleware authenticates bearer tokens before entering protected handlers.
func AuthMiddleware(verifier Verifier, allowViewAsAdmin bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, err := bearerToken(r)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "unauthorized", err.Error())
				return
			}

			viewer, err := verifier.Verify(r.Context(), token)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired access token")
				return
			}

			viewer = WithViewRole(viewer, r.Header.Get(ViewRoleHeader), allowViewAsAdmin)
			next.ServeHTTP(w, r.WithContext(WithViewer(r.Context(), viewer)))
		})
	}
}

func bearerToken(r *http.Request) (string, error) {
	const prefix = "Bearer "

	header := r.Header.Get("Authorization")
	if len(header) <= len(prefix) || header[:len(prefix)] != prefix {
		return "", fmt.Errorf("missing bearer token")
	}

	return header[len(prefix):], nil
}
