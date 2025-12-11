package gamify

import (
	"context"
	"testing"

	mem "gamifykit/adapters/memory"
	"gamifykit/core"
	"gamifykit/engine"
	"gamifykit/realtime"
)

func TestNewDefaultsAndOptions(t *testing.T) {
	hub := realtime.NewHub()
	svc := New(
		WithRealtime(hub),
		WithStorage(mem.New()),
		WithDispatchMode(engine.DispatchSync),
	)

	// basic operation
	total, err := svc.AddPoints(context.Background(), "alice", core.MetricXP, 5)
	if err != nil || total != 5 {
		t.Fatalf("add points total=%d err=%v", total, err)
	}

	// realtime bridge should receive event
	_, ch := hub.Subscribe(1)
	svc.Publish(context.Background(), core.NewPointsAdded("alice", core.MetricXP, 5, 10))
	ev := <-ch
	if ev.UserID != "alice" || ev.Type != core.EventPointsAdded {
		t.Fatalf("unexpected event: %+v", ev)
	}
}

func TestInMemoryFallback(t *testing.T) {
	svc := New()
	if _, err := svc.AddPoints(context.Background(), "bob", core.MetricXP, 3); err != nil {
		t.Fatalf("fallback add points: %v", err)
	}
	state, err := svc.GetState(context.Background(), "bob")
	if err != nil {
		t.Fatalf("fallback get state: %v", err)
	}
	if state.Points[core.MetricXP] != 3 {
		t.Fatalf("expected 3 points, got %d", state.Points[core.MetricXP])
	}
}
