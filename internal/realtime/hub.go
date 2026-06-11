package realtime

import (
	"encoding/json"
	"sync"

	"taskboard-backend/internal/tasks"
)

// EventType identifies a realtime task event.
type EventType string

const (
	EventTaskCreated EventType = "task.created"
	EventTaskUpdated EventType = "task.updated"
	EventTaskDeleted EventType = "task.deleted"
)

// Event is broadcast to connected clients.
type Event struct {
	Type   EventType   `json:"type"`
	Task   *tasks.Task `json:"task,omitempty"`
	TaskID string      `json:"task_id"`
}

// Hub fans out task events to subscribed clients.
type Hub struct {
	mu          sync.RWMutex
	subscribers map[chan Event]subscriberMeta
}

type subscriberMeta struct {
	userID  string
	isAdmin bool
}

// NewHub creates an in-memory realtime hub.
func NewHub() *Hub {
	return &Hub{
		subscribers: make(map[chan Event]subscriberMeta),
	}
}

// Subscribe registers a client for realtime events.
func (h *Hub) Subscribe(userID string, isAdmin bool) chan Event {
	ch := make(chan Event, 8)

	h.mu.Lock()
	h.subscribers[ch] = subscriberMeta{userID: userID, isAdmin: isAdmin}
	h.mu.Unlock()

	return ch
}

// Unsubscribe removes a client from the hub.
func (h *Hub) Unsubscribe(ch chan Event) {
	h.mu.Lock()
	delete(h.subscribers, ch)
	h.mu.Unlock()
	close(ch)
}

// Broadcast sends an event to the task owner and all admin subscribers.
func (h *Hub) Broadcast(event Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	ownerID := ""
	if event.Task != nil {
		ownerID = event.Task.UserID
	}

	for ch, meta := range h.subscribers {
		if meta.isAdmin || (ownerID != "" && meta.userID == ownerID) {
			select {
			case ch <- event:
			default:
			}
		}
	}
}

// Marshal encodes an event for SSE delivery.
func (e Event) Marshal() ([]byte, error) {
	return json.Marshal(e)
}
