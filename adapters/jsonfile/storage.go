package jsonfile

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gamifykit/core"
)

// Store persists entire state to a single JSON file.
// Suitable for demos and small deployments.
type Store struct {
	path string
	mu   sync.Mutex
	// in-memory cache for speed
	data map[core.UserID]core.UserState
}

func New(path string) (*Store, error) {
	s := &Store{path: path, data: map[core.UserID]core.UserState{}}
	if err := s.load(); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	return s, nil
}

func (s *Store) load() error {
	b, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	var raw map[string]core.UserState
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	for k, v := range raw {
		s.data[core.UserID(k)] = v
	}
	return nil
}

func (s *Store) persist() error {
	tmp := s.path + ".tmp"
	raw := make(map[string]core.UserState, len(s.data))
	for k, v := range s.data {
		raw[string(k)] = v
	}
	b, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) get(user core.UserID) core.UserState {
	if st, ok := s.data[user]; ok {
		return st
	}
	st := core.UserState{UserID: user, Points: map[core.Metric]int64{}, Badges: map[core.Badge]struct{}{}, Levels: map[core.Metric]int64{}, Updated: time.Now().UTC()}
	s.data[user] = st
	return st
}

func (s *Store) AddPoints(_ context.Context, user core.UserID, metric core.Metric, delta int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.get(user)
	next, err := core.AddSafe(st.Points[metric], delta)
	if err != nil {
		return 0, err
	}
	st.Points[metric] = next
	st.Updated = time.Now().UTC()
	s.data[user] = st
	if err := s.persist(); err != nil {
		return 0, err
	}
	return next, nil
}

func (s *Store) AwardBadge(_ context.Context, user core.UserID, badge core.Badge) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.get(user)
	st.Badges[badge] = struct{}{}
	st.Updated = time.Now().UTC()
	s.data[user] = st
	return s.persist()
}

func (s *Store) GetState(_ context.Context, user core.UserID) (core.UserState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.get(user)
	return st.Clone(), nil
}

func (s *Store) SetLevel(_ context.Context, user core.UserID, metric core.Metric, level int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.get(user)
	st.Levels[metric] = level
	st.Updated = time.Now().UTC()
	s.data[user] = st
	return s.persist()
}
