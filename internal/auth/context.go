package auth

import "context"

type viewerKey struct{}

// Viewer contains the authenticated user context used by handlers.
type Viewer struct {
	UserID     string
	Email      string
	ActualRole string
	Role       string
}

// IsAdmin reports whether the viewer is operating in admin view.
func (v Viewer) IsAdmin() bool {
	return v.Role == "admin"
}

// HasAdminGrant reports whether the JWT grants admin privileges.
func (v Viewer) HasAdminGrant() bool {
	return v.ActualRole == "admin"
}

// WithViewer stores the viewer in the request context.
func WithViewer(ctx context.Context, viewer Viewer) context.Context {
	return context.WithValue(ctx, viewerKey{}, viewer)
}

// ViewerFromContext extracts the viewer from context.
func ViewerFromContext(ctx context.Context) (Viewer, bool) {
	viewer, ok := ctx.Value(viewerKey{}).(Viewer)
	return viewer, ok
}
