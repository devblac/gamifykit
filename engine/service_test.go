package engine

import (
	"context"
	"testing"

	mem "gamifykit/adapters/memory"
	"gamifykit/core"
)

func TestAddPointsAndLevelUp(t *testing.T) {
	store := mem.New()
	bus := NewEventBus(DispatchSync)
	svc := NewGamifyService(store, bus, DefaultRuleEngine())

	levelUp := 0
	svc.Subscribe(core.EventLevelUp, func(ctx context.Context, e core.Event) { levelUp++ })

	total, err := svc.AddPoints(context.Background(), core.UserID("user1"), core.MetricXP, 10000)
	if err != nil {
		t.Fatal(err)
	}
	if total <= 0 {
		t.Fatal("total should be > 0")
	}
	if levelUp == 0 {
		t.Fatal("expected level up event")
	}
}
