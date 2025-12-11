package websocket

import (
	"net/http"
	"time"

	"gamifykit/realtime"
	gorillaws "github.com/gorilla/websocket"
)

// Handler returns an http.Handler that upgrades to WebSocket and streams events from the hub.
func Handler(hub *realtime.Hub) http.Handler {
	upgrader := gorillaws.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		id, ch := hub.Subscribe(256)
		defer hub.Unsubscribe(id)

		_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		for ev := range ch {
			if err := conn.WriteMessage(gorillaws.TextMessage, realtime.MarshalJSON(ev)); err != nil {
				return
			}
		}
	})
}
