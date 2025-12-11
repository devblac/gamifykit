package gamify

import (
	"context"

	"gamifykit/core"
	"gamifykit/engine"
	"gamifykit/realtime"
)

// Option configures the Gamify service builder.
type Option func(*config)

type config struct {
	storage engine.Storage
	mode    engine.DispatchMode
	rules   engine.RuleEngine
	hub     *realtime.Hub
}

// WithStorage sets the persistence adapter.
func WithStorage(s engine.Storage) Option { return func(c *config) { c.storage = s } }

// WithRuleEngine sets the rule engine.
func WithRuleEngine(r engine.RuleEngine) Option { return func(c *config) { c.rules = r } }

// WithDispatchMode selects sync or async event dispatch.
func WithDispatchMode(m engine.DispatchMode) Option { return func(c *config) { c.mode = m } }

// WithRealtime wires a realtime hub to receive all engine events.
func WithRealtime(h *realtime.Hub) Option { return func(c *config) { c.hub = h } }

// New builds a configured GamifyService. If not provided, defaults are used:
//  - storage: in-memory
//  - rules: DefaultRuleEngine
//  - dispatch: async
func New(opts ...Option) *engine.GamifyService {
	cfg := &config{mode: engine.DispatchAsync, rules: engine.DefaultRuleEngine()}
	for _, o := range opts {
		o(cfg)
	}
	if cfg.storage == nil {
		// lazy import via interface to avoid cycle; implementors should pass explicit storage in prod
		cfg.storage = &inMemoryFallback{}
	}
	bus := engine.NewEventBus(cfg.mode)
	svc := engine.NewGamifyService(cfg.storage, bus, cfg.rules)
	if cfg.hub != nil {
		// Bridge all primary events to realtime
		bus.Subscribe(core.EventPointsAdded, func(ctx context.Context, e core.Event) { cfg.hub.Broadcast(ctx, e) })
		bus.Subscribe(core.EventLevelUp, func(ctx context.Context, e core.Event) { cfg.hub.Broadcast(ctx, e) })
		bus.Subscribe(core.EventBadgeAwarded, func(ctx context.Context, e core.Event) { cfg.hub.Broadcast(ctx, e) })
		bus.Subscribe(core.EventAchievementUnlocked, func(ctx context.Context, e core.Event) { cfg.hub.Broadcast(ctx, e) })
	}
	return svc
}

// inMemoryFallback is a tiny local storage to keep New() usable without external deps.
type inMemoryFallback struct{ mem engine.Storage }

func (m *inMemoryFallback) ensure() engine.Storage {
	if m.mem == nil {
		m.mem = &memStore{}
	}
	return m.mem
}

func (m *inMemoryFallback) AddPoints(ctx context.Context, u core.UserID, metric core.Metric, d int64) (int64, error) {
	return m.ensure().AddPoints(ctx, u, metric, d)
}
func (m *inMemoryFallback) AwardBadge(ctx context.Context, u core.UserID, b core.Badge) error {
	return m.ensure().AwardBadge(ctx, u, b)
}
func (m *inMemoryFallback) GetState(ctx context.Context, u core.UserID) (core.UserState, error) {
	return m.ensure().GetState(ctx, u)
}
func (m *inMemoryFallback) SetLevel(ctx context.Context, u core.UserID, metric core.Metric, lvl int64) error {
	return m.ensure().SetLevel(ctx, u, metric, lvl)
}

// minimal memory impl mirroring adapters/memory to avoid import cycle.
type memStore struct {
	data map[core.UserID]core.UserState
}

func (s *memStore) ensure(u core.UserID) core.UserState {
	if s.data == nil {
		s.data = map[core.UserID]core.UserState{}
	}
	if st, ok := s.data[u]; ok {
		return st
	}
	st := core.UserState{UserID: u, Points: map[core.Metric]int64{}, Badges: map[core.Badge]struct{}{}, Levels: map[core.Metric]int64{}}
	s.data[u] = st
	return st
}

func (s *memStore) AddPoints(_ context.Context, u core.UserID, metric core.Metric, d int64) (int64, error) {
	st := s.ensure(u)
	next, err := core.AddSafe(st.Points[metric], d)
	if err != nil {
		return 0, err
	}
	st.Points[metric] = next
	s.data[u] = st
	return next, nil
}
func (s *memStore) AwardBadge(_ context.Context, u core.UserID, b core.Badge) error {
	st := s.ensure(u)
	st.Badges[b] = struct{}{}
	s.data[u] = st
	return nil
}
func (s *memStore) GetState(_ context.Context, u core.UserID) (core.UserState, error) {
	return s.ensure(u).Clone(), nil
}
func (s *memStore) SetLevel(_ context.Context, u core.UserID, metric core.Metric, lvl int64) error {
	st := s.ensure(u)
	st.Levels[metric] = lvl
	s.data[u] = st
	return nil
}
