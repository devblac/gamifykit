package memory

import (
	"context"
	"sync"
	"time"

	"gamifykit/core"
)

// Store is a concurrent in-memory Storage implementation.
type Store struct {
	users sync.Map // map[core.UserID]*userRecord
}

type userRecord struct {
	mu    sync.Mutex
	state core.UserState
}

func New() *Store { return &Store{} }

func (s *Store) getOrCreate(user core.UserID) *userRecord {
	if v, ok := s.users.Load(user); ok {
		return v.(*userRecord)
	}
	rec := &userRecord{state: core.UserState{
		UserID:  user,
		Points:  map[core.Metric]int64{},
		Badges:  map[core.Badge]struct{}{},
		Levels:  map[core.Metric]int64{},
		Updated: time.Now().UTC(),
	}}
	actual, _ := s.users.LoadOrStore(user, rec)
	return actual.(*userRecord)
}

func (s *Store) AddPoints(_ context.Context, user core.UserID, metric core.Metric, delta int64) (int64, error) {
	rec := s.getOrCreate(user)
	rec.mu.Lock()
	defer rec.mu.Unlock()
	current := rec.state.Points[metric]
	next, err := core.AddSafe(current, delta)
	if err != nil {
		return 0, err
	}
	rec.state.Points[metric] = next
	rec.state.Updated = time.Now().UTC()
	return next, nil
}

func (s *Store) AwardBadge(_ context.Context, user core.UserID, badge core.Badge) error {
	rec := s.getOrCreate(user)
	rec.mu.Lock()
	defer rec.mu.Unlock()
	rec.state.Badges[badge] = struct{}{}
	rec.state.Updated = time.Now().UTC()
	return nil
}

func (s *Store) GetState(_ context.Context, user core.UserID) (core.UserState, error) {
	rec := s.getOrCreate(user)
	rec.mu.Lock()
	defer rec.mu.Unlock()
	return rec.state.Clone(), nil
}

func (s *Store) SetLevel(_ context.Context, user core.UserID, metric core.Metric, level int64) error {
	rec := s.getOrCreate(user)
	rec.mu.Lock()
	defer rec.mu.Unlock()
	rec.state.Levels[metric] = level
	rec.state.Updated = time.Now().UTC()
	return nil
}

var _ interface {
	AddPoints(context.Context, core.UserID, core.Metric, int64) (int64, error)
	AwardBadge(context.Context, core.UserID, core.Badge) error
	GetState(context.Context, core.UserID) (core.UserState, error)
	SetLevel(context.Context, core.UserID, core.Metric, int64) error
} = (*Store)(nil)
