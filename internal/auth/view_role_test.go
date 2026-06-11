package auth

import "testing"

func TestWithViewRoleAdminCanSwitchToUser(t *testing.T) {
	t.Parallel()

	viewer := Viewer{
		UserID:     "admin-1",
		Email:      "admin@example.com",
		ActualRole: "admin",
		Role:       "admin",
	}

	switched := WithViewRole(viewer, "user", false)
	if switched.IsAdmin() {
		t.Fatal("expected admin viewer to switch to user view")
	}
	if switched.ActualRole != "admin" {
		t.Fatalf("expected actual role to remain admin, got %q", switched.ActualRole)
	}
}

func TestWithViewRoleNonAdminCannotElevateWithoutPermission(t *testing.T) {
	t.Parallel()

	viewer := Viewer{
		UserID:     "user-1",
		Email:      "user@example.com",
		ActualRole: "authenticated",
		Role:       "authenticated",
	}

	switched := WithViewRole(viewer, "admin", false)
	if switched.IsAdmin() {
		t.Fatal("expected non-admin viewer to remain non-admin")
	}
}

func TestWithViewRoleNonAdminCanElevateWhenAllowed(t *testing.T) {
	t.Parallel()

	viewer := Viewer{
		UserID:     "user-1",
		Email:      "user@example.com",
		ActualRole: "authenticated",
		Role:       "authenticated",
	}

	switched := WithViewRole(viewer, "admin", true)
	if !switched.IsAdmin() {
		t.Fatal("expected viewer-as-admin when toggle is allowed")
	}
}
