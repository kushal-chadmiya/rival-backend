package tasks

import (
	"net/url"
	"testing"
)

func TestParseListParams(t *testing.T) {
	t.Parallel()

	params, err := ParseListParams(url.Values{
		"status":    []string{"todo"},
		"search":    []string{"release"},
		"sort_by":   []string{"due_date"},
		"sort_dir":  []string{"asc"},
		"page":      []string{"2"},
		"page_size": []string{"20"},
	})
	if err != nil {
		t.Fatalf("ParseListParams returned error: %v", err)
	}

	if params.Status != "todo" || params.Search != "release" || params.SortBy != "due_date" || params.SortDir != "asc" || params.Page != 2 || params.PageSize != 20 {
		t.Fatalf("unexpected parsed params: %#v", params)
	}
}

func TestValidateCreateRejectsBadPayload(t *testing.T) {
	t.Parallel()

	_, err := ValidateCreate(CreateTaskInput{
		Title:    "",
		Status:   "done",
		Priority: "urgent",
		DueDate:  "today",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateTaskID(t *testing.T) {
	t.Parallel()

	if err := ValidateTaskID("not-a-uuid"); err == nil {
		t.Fatal("expected invalid uuid error")
	}
	if err := ValidateTaskID("8dba5f0e-f34f-4c69-b4d4-94bf304bc4de"); err != nil {
		t.Fatalf("expected valid uuid, got %v", err)
	}
}

func TestValidateUpdateParsesDueDateAndTrimsTitle(t *testing.T) {
	t.Parallel()

	title := "  Ship API  "
	dueDate := "2026-06-30T10:00:00Z"

	params, err := ValidateUpdate(UpdateTaskInput{
		Title:   &title,
		DueDate: &dueDate,
	})
	if err != nil {
		t.Fatalf("ValidateUpdate returned error: %v", err)
	}

	if params.Title == nil || *params.Title != "Ship API" {
		t.Fatalf("expected trimmed title, got %#v", params.Title)
	}
	if params.DueDate == nil || params.DueDate.Format("2006-01-02") != "2026-06-30" {
		t.Fatalf("expected parsed due date, got %#v", params.DueDate)
	}
}
