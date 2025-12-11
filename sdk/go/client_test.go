package sdk

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"gamifykit/core"
)

func TestClient_AddPointsAwardBadgeGetUserHealth(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	client, err := NewClient(srv.URL+"/api", WithAPIKey("k1"))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	ctx := context.Background()

	total, err := client.AddPoints(ctx, "alice", 50, "xp")
	if err != nil || total != 50 {
		t.Fatalf("add points got total=%d err=%v", total, err)
	}

	if err := client.AwardBadge(ctx, "alice", "onboarded"); err != nil {
		t.Fatalf("award badge: %v", err)
	}

	state, err := client.GetUser(ctx, "alice")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if state.UserID != "alice" || state.Points["xp"] != 50 {
		t.Fatalf("unexpected state: %+v", state)
	}

	health, err := client.Health(ctx)
	if err != nil || health.Status != "healthy" {
		t.Fatalf("health: %+v err=%v", health, err)
	}
}

func TestClient_SubscribeEvents(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	client, err := NewClient(srv.URL + "/api")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	events, err := client.SubscribeEvents(ctx)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	select {
	case evt := <-events:
		if evt.Type != core.EventPointsAdded {
			t.Fatalf("unexpected event type: %s", evt.Type)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for event")
	}
}

// test server implementing the minimal API surface expected by the SDK.
func newTestServer() *httptest.Server {
	var points int64

	mux := http.NewServeMux()
	mux.HandleFunc("/api/healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"healthy","checks":{"storage":"ok"}}`))
	})
	mux.HandleFunc("/api/users/", func(w http.ResponseWriter, r *http.Request) {
		// /api/users/{id}[/points|/badges/{badge}]
		path := r.URL.Path[len("/api/users/"):]
		parts := strings.Split(path, "/")
		if len(parts) == 0 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		userID := parts[0]
		if len(parts) == 1 && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"user_id":"` + userID + `","points":{"xp":50},"badges":{},"levels":{}}`))
			return
		}
		if len(parts) >= 2 && parts[1] == "points" && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			points += 50
			_, _ = w.Write([]byte(`{"total":50}`))
			return
		}
		if len(parts) >= 3 && parts[1] == "badges" && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	upgrader := websocket.Upgrader{}
	mux.HandleFunc("/api/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		evt := core.NewPointsAdded("alice", core.MetricXP, 10, 10)
		_ = conn.WriteJSON(evt)
	})

	return httptest.NewServer(mux)
}
