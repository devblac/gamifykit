package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strconv"

	mem "gamifykit/adapters/memory"
	ws "gamifykit/adapters/websocket"
	"gamifykit/core"
	"gamifykit/engine"
	"gamifykit/realtime"
)

func main() {
	// Use readable text logging for development/demo
	textHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	slog.SetDefault(slog.New(textHandler))

	ctx := context.Background()
	store := mem.New()
	bus := engine.NewEventBus(engine.DispatchAsync)
	svc := engine.NewGamifyService(store, bus, engine.DefaultRuleEngine())
	hub := realtime.NewHub()

	// Forward gamification events to WebSocket clients
	bus.Subscribe(core.EventPointsAdded, func(ctx context.Context, e core.Event) { hub.Broadcast(ctx, e) })
	bus.Subscribe(core.EventLevelUp, func(ctx context.Context, e core.Event) { hub.Broadcast(ctx, e) })
	bus.Subscribe(core.EventBadgeAwarded, func(ctx context.Context, e core.Event) { hub.Broadcast(ctx, e) })

	http.Handle("/ws", ws.Handler(hub))
	http.HandleFunc("/users/", func(w http.ResponseWriter, r *http.Request) {
		// routes: /users/{id}/points?metric=xp&delta=50, /users/{id}/badges/{badge}, GET /users/{id}
		parts := split(r.URL.Path, '/')
		if len(parts) < 2 {
			http.NotFound(w, r)
			return
		}
		user := core.UserID(parts[1])
		switch r.Method {
		case http.MethodPost:
			if len(parts) >= 3 && parts[2] == "points" {
				metric := core.Metric(r.URL.Query().Get("metric"))
				if metric == "" {
					metric = core.MetricXP
				}
				delta, _ := strconv.ParseInt(r.URL.Query().Get("delta"), 10, 64)
				total, err := svc.AddPoints(ctx, user, metric, delta)
				writeJSON(w, map[string]any{"total": total, "err": errString(err)})
				return
			}
			if len(parts) >= 4 && parts[2] == "badges" {
				badge := core.Badge(parts[3])
				err := svc.AwardBadge(ctx, user, badge)
				writeJSON(w, map[string]any{"ok": err == nil, "err": errString(err)})
				return
			}
		case http.MethodGet:
			st, err := svc.GetState(ctx, user)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			writeJSON(w, st)
			return
		}
		http.NotFound(w, r)
	})

	slog.Info("starting demo server on :8080")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		slog.Error("demo server crashed", "error", err)
		os.Exit(1)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func errString(err error) any {
	if err == nil {
		return nil
	}
	return err.Error()
}

func split(p string, sep rune) []string {
	var parts []string
	current := make([]rune, 0, len(p))

	for _, r := range p {
		if r == sep {
			if len(current) > 0 {
				parts = append(parts, string(current))
				current = current[:0]
			}
			continue
		}
		current = append(current, r)
	}

	if len(current) > 0 {
		parts = append(parts, string(current))
	}

	return parts
}
