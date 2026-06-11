package auth

import "strings"

const ViewRoleHeader = "X-View-Role"

// WithViewRole applies the requested view role on top of the JWT role.
func WithViewRole(viewer Viewer, viewRole string, allowViewAsAdmin bool) Viewer {
	actual := viewer.ActualRole
	if actual == "" {
		actual = viewer.Role
	}

	effective := actual
	switch strings.ToLower(strings.TrimSpace(viewRole)) {
	case "admin":
		effective = "admin"
	case "user", "authenticated":
		if actual == "admin" {
			effective = "authenticated"
		} else {
			effective = actual
		}
	}

	if effective == "admin" && actual != "admin" && !allowViewAsAdmin {
		effective = actual
	}

	return Viewer{
		UserID:     viewer.UserID,
		Email:      viewer.Email,
		ActualRole: actual,
		Role:       effective,
	}
}
