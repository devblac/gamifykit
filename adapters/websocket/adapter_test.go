package websocket

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	gorillaws "github.com/gorilla/websocket"

	"gamifykit/core"
	"gamifykit/realtime"
)

func TestHandlerStreamsEvents(t *testing.T) {
	hub := realtime.NewHub()
	server := httptest.NewServer(Handler(hub))
	defer server.Close()

	wsURL := "ws" + server.URL[len("http"):] // convert http->ws
	conn, _, err := gorillaws.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial ws: %v", err)
	}
	defer conn.Close()

	// ensure subscriber goroutine is ready
	time.Sleep(10 * time.Millisecond)

	ev := core.NewPointsAdded("alice", core.MetricXP, 5, 5)
	hub.Broadcast(context.Background(), ev)

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read message: %v", err)
	}

	var received core.Event
	if err := json.Unmarshal(msg, &received); err != nil {
		t.Fatalf("decode event: %v", err)
	}
	if received.UserID != "alice" {
		t.Fatalf("unexpected user: %s", received.UserID)
	}
}
