package realtime

import (
	"context"
	"encoding/json"
	"testing"

	"gamifykit/core"
)

func TestHubSubscribeBroadcastUnsubscribe(t *testing.T) {
	h := NewHub()
	id, ch := h.Subscribe(1)

	ev := core.NewPointsAdded("bob", core.MetricXP, 10, 10)
	h.Broadcast(context.Background(), ev)

	received := <-ch
	if received.UserID != "bob" || received.Type != core.EventPointsAdded {
		t.Fatalf("unexpected event: %+v", received)
	}

	h.Unsubscribe(id)
	_, ok := <-ch
	if ok {
		t.Fatal("expected channel closed after unsubscribe")
	}
}

func TestMarshalJSON(t *testing.T) {
	ev := core.NewBadgeAwarded("alice", "onboarded")
	b := MarshalJSON(ev)
	var out core.Event
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Badge != "onboarded" {
		t.Fatalf("unexpected badge: %s", out.Badge)
	}
}
