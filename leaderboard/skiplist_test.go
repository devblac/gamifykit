package leaderboard

import (
	"gamifykit/core"
	"testing"
)

func TestSkipListBasic(t *testing.T) {
	s := NewSkipList()
	s.Update(core.UserID("a"), 10)
	s.Update(core.UserID("b"), 20)
	s.Update(core.UserID("c"), 15)
	top := s.TopN(3)
	if len(top) != 3 || top[0].User != core.UserID("b") || top[1].User != core.UserID("c") || top[2].User != core.UserID("a") {
		t.Fatalf("unexpected order: %#v", top)
	}
	s.Update(core.UserID("a"), 25)
	top = s.TopN(1)
	if top[0].User != core.UserID("a") {
		t.Fatalf("top should be a, got %#v", top)
	}
}
