package realtime

import (
	"context"
	"encoding/json"
	"sync"

	"gamifykit/core"
)

// Hub is a simple pub/sub for broadcasting events to channels.
type Hub struct {
	mu   sync.RWMutex
	subs map[int]chan core.Event
	next int
}

func NewHub() *Hub { return &Hub{subs: map[int]chan core.Event{}} }

func (h *Hub) Subscribe(buffer int) (int, <-chan core.Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.next++
	id := h.next
	ch := make(chan core.Event, buffer)
	h.subs[id] = ch
	return id, ch
}

func (h *Hub) Unsubscribe(id int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if ch, ok := h.subs[id]; ok {
		delete(h.subs, id)
		close(ch)
	}
}

func (h *Hub) Broadcast(_ context.Context, ev core.Event) {
	h.mu.RLock()
	// copy to avoid holding lock during send
	receivers := make([]chan core.Event, 0, len(h.subs))
	for _, ch := range h.subs {
		receivers = append(receivers, ch)
	}
	h.mu.RUnlock()
	for _, ch := range receivers {
		select {
		case ch <- ev:
		default: /* drop if full */
		}
	}
}

// MarshalJSON is a helper to convert events to JSON bytes for WebSocket/SSE.
func MarshalJSON(ev core.Event) []byte {
	b, _ := json.Marshal(ev)
	return b
}
