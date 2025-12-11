package engine

import (
	"context"
	"testing"
	"time"

	"gamifykit/core"
)

func TestEventBusSync(t *testing.T) {
	bus := NewEventBus(DispatchSync)
	count := 0
	bus.Subscribe(core.EventPointsAdded, func(ctx context.Context, e core.Event) { count++ })
	bus.Publish(context.Background(), core.NewPointsAdded(core.UserID("u"), core.MetricXP, 1, 1))
	if count != 1 {
		t.Fatalf("want 1 got %d", count)
	}
}

func TestEventBusAsync(t *testing.T) {
	bus := NewEventBus(DispatchAsync)
	defer bus.Close()
	ch := make(chan struct{})
	bus.Subscribe(core.EventPointsAdded, func(ctx context.Context, e core.Event) { close(ch) })
	bus.Publish(context.Background(), core.NewPointsAdded(core.UserID("u"), core.MetricXP, 1, 1))
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}
