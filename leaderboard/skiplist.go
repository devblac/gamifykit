package leaderboard

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	"math/rand/v2"
	"sync"

	"gamifykit/core"
)

// A simple skip list keyed by (score desc, user asc) to achieve O(log n) updates.

const maxLevel = 16
const pFactor = 0.25

type node struct {
	e    Entry
	next [maxLevel]*node
}

type SkipList struct {
	mu     sync.RWMutex
	head   *node
	lvl    int
	byUser map[core.UserID]*node
	rng    *rand.Rand
}

func NewSkipList() *SkipList {
	// Use crypto/rand to generate a secure seed for PCG
	var seed [16]byte
	if _, err := cryptorand.Read(seed[:]); err != nil {
		// Fallback to zero seed if crypto/rand fails (extremely unlikely)
		seed = [16]byte{}
	}
	seed1 := binary.BigEndian.Uint64(seed[:8])
	seed2 := binary.BigEndian.Uint64(seed[8:])

	return &SkipList{
		head:   &node{},
		lvl:    1,
		byUser: map[core.UserID]*node{},
		rng:    rand.New(rand.NewPCG(seed1, seed2)),
	}
}

func (s *SkipList) randomLevel() int {
	lvl := 1
	for lvl < maxLevel && s.rng.Float64() < pFactor {
		lvl++
	}
	return lvl
}

func less(a, b Entry) bool {
	if a.Score == b.Score {
		return a.User < b.User
	}
	return a.Score > b.Score // higher score first
}

// Update inserts or moves user to new score.
func (s *SkipList) Update(user core.UserID, score int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.byUser[user]; ok {
		// remove old node
		s.removeLocked(user, old.e)
	}
	e := Entry{User: user, Score: score}
	update := [maxLevel]*node{}
	cur := s.head
	for i := s.lvl - 1; i >= 0; i-- {
		for cur.next[i] != nil && less(cur.next[i].e, e) {
			cur = cur.next[i]
		}
		update[i] = cur
	}
	lvl := s.randomLevel()
	if lvl > s.lvl {
		for i := s.lvl; i < lvl; i++ {
			update[i] = s.head
		}
		s.lvl = lvl
	}
	n := &node{e: e}
	for i := 0; i < lvl; i++ {
		n.next[i] = update[i].next[i]
		update[i].next[i] = n
	}
	s.byUser[user] = n
}

func (s *SkipList) removeLocked(user core.UserID, e Entry) {
	update := [maxLevel]*node{}
	cur := s.head
	for i := s.lvl - 1; i >= 0; i-- {
		for cur.next[i] != nil && less(cur.next[i].e, e) {
			cur = cur.next[i]
		}
		update[i] = cur
	}
	target := update[0].next[0]
	if target == nil || target.e.User != user {
		return
	}
	for i := 0; i < s.lvl; i++ {
		if update[i].next[i] == target {
			update[i].next[i] = target.next[i]
		}
	}
	delete(s.byUser, user)
	for s.lvl > 1 && s.head.next[s.lvl-1] == nil {
		s.lvl--
	}
}

func (s *SkipList) Remove(user core.UserID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n, ok := s.byUser[user]; ok {
		s.removeLocked(user, n.e)
	}
}

func (s *SkipList) TopN(n int) []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if n <= 0 {
		return nil
	}
	out := make([]Entry, 0, n)
	cur := s.head.next[0]
	for cur != nil && len(out) < n {
		out = append(out, cur.e)
		cur = cur.next[0]
	}
	return out
}

func (s *SkipList) Get(user core.UserID) (Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if n, ok := s.byUser[user]; ok {
		return n.e, true
	}
	return Entry{}, false
}

var _ Board = (*SkipList)(nil)
