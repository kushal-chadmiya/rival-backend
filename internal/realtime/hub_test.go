package realtime

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"taskboard-backend/internal/tasks"
)

func TestHubBroadcastsToOwnerAndAdmin(t *testing.T) {
	t.Parallel()

	hub := NewHub()
	ownerCh := hub.Subscribe("owner-1", false)
	adminCh := hub.Subscribe("admin-1", true)
	otherCh := hub.Subscribe("other-2", false)
	defer hub.Unsubscribe(ownerCh)
	defer hub.Unsubscribe(adminCh)
	defer hub.Unsubscribe(otherCh)

	task := tasks.Task{
		ID:     uuid.New(),
		UserID: "owner-1",
		Title:  "Shared task",
	}

	hub.Broadcast(Event{
		Type:   EventTaskUpdated,
		Task:   &task,
		TaskID: task.ID.String(),
	})

	select {
	case event := <-ownerCh:
		if event.Type != EventTaskUpdated {
			t.Fatalf("unexpected owner event: %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("owner did not receive event")
	}

	select {
	case <-adminCh:
	case <-time.After(time.Second):
		t.Fatal("admin did not receive event")
	}

	select {
	case <-otherCh:
		t.Fatal("other user should not receive event")
	case <-time.After(100 * time.Millisecond):
	}
}
