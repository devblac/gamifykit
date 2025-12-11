package memory

import (
	"context"
	"gamifykit/core"
	"testing"
)

func TestMemoryStore(t *testing.T) {
	s := New()
	total, err := s.AddPoints(context.Background(), core.UserID("u"), core.MetricXP, 5)
	if err != nil || total != 5 {
		t.Fatalf("got %v %v", total, err)
	}
	if err := s.AwardBadge(context.Background(), core.UserID("u"), core.Badge("starter")); err != nil {
		t.Fatal(err)
	}
	st, _ := s.GetState(context.Background(), core.UserID("u"))
	if _, ok := st.Badges[core.Badge("starter")]; !ok {
		t.Fatal("badge missing")
	}
}
