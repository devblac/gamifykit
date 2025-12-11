package jsonfile

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"gamifykit/core"
)

func TestStorePersistAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	store, err := New(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	total, err := store.AddPoints(context.Background(), "alice", core.MetricXP, 50)
	if err != nil || total != 50 {
		t.Fatalf("add points: total=%d err=%v", total, err)
	}

	if err := store.AwardBadge(context.Background(), "alice", "onboarded"); err != nil {
		t.Fatalf("award badge: %v", err)
	}
	if err := store.SetLevel(context.Background(), "alice", core.MetricXP, 2); err != nil {
		t.Fatalf("set level: %v", err)
	}

	// ensure file written
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file at %s", path)
	}

	// reload
	reloaded, err := New(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}

	state, err := reloaded.GetState(context.Background(), "alice")
	if err != nil {
		t.Fatalf("get state: %v", err)
	}
	if state.Points[core.MetricXP] != 50 {
		t.Fatalf("expected points 50, got %d", state.Points[core.MetricXP])
	}
	if _, ok := state.Badges[core.Badge("onboarded")]; !ok {
		t.Fatalf("expected badge onboarded")
	}
	if state.Levels[core.MetricXP] != 2 {
		t.Fatalf("expected level 2, got %d", state.Levels[core.MetricXP])
	}
}
