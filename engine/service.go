package engine

import (
	"context"
	"errors"

	"gamifykit/core"
)

// GamifyService wires storage, event bus, and rules into a cohesive API.
type GamifyService struct {
	storage Storage
	bus     *EventBus
	rules   RuleEngine
}

func NewGamifyService(storage Storage, bus *EventBus, rules RuleEngine) *GamifyService {
	if storage == nil || bus == nil || rules == nil {
		panic("NewGamifyService requires non-nil storage, bus, and rules")
	}
	return &GamifyService{storage: storage, bus: bus, rules: rules}
}

func DefaultRuleEngine() RuleEngine {
	return &simpleRuleEngine{rules: []core.Rule{core.LevelUpRule{Metric: core.MetricXP}}}
}

// Subscribe convenience method.
func (g *GamifyService) Subscribe(typ core.EventType, handler func(context.Context, core.Event)) func() {
	return g.bus.Subscribe(typ, handler)
}

func (g *GamifyService) Publish(ctx context.Context, ev core.Event) {
	g.bus.Publish(ctx, ev)
}

func (g *GamifyService) AddPoints(ctx context.Context, user core.UserID, metric core.Metric, delta int64) (int64, error) {
	if delta == 0 {
		return 0, errors.New("delta cannot be zero")
	}
	normalized, err := core.NormalizeUserID(user)
	if err != nil {
		return 0, err
	}
	total, err := g.storage.AddPoints(ctx, normalized, metric, delta)
	if err != nil {
		return 0, err
	}
	ev := core.NewPointsAdded(normalized, metric, delta, total)
	g.bus.Publish(ctx, ev)
	state, err := g.storage.GetState(ctx, normalized)
	if err == nil {
		derived := g.rules.Evaluate(ctx, state, ev)
		for _, d := range derived {
			// allow rules to update storage when needed
			if d.Type == core.EventLevelUp {
				_ = g.storage.SetLevel(ctx, d.UserID, d.Metric, d.Level)
			}
			g.bus.Publish(ctx, d)
		}
	}
	return total, nil
}

func (g *GamifyService) AwardBadge(ctx context.Context, user core.UserID, badge core.Badge) error {
	normalized, err := core.NormalizeUserID(user)
	if err != nil {
		return err
	}
	if err := core.ValidateBadgeID(badge); err != nil {
		return err
	}
	if err := g.storage.AwardBadge(ctx, normalized, badge); err != nil {
		return err
	}
	g.bus.Publish(ctx, core.NewBadgeAwarded(normalized, badge))
	return nil
}

func (g *GamifyService) EvaluateRules(ctx context.Context, user core.UserID) error {
	state, err := g.storage.GetState(ctx, user)
	if err != nil {
		return err
	}
	// no specific trigger; allow engines to infer
	derived := g.rules.Evaluate(ctx, state, core.Event{UserID: user})
	for _, d := range derived {
		if d.Type == core.EventLevelUp {
			_ = g.storage.SetLevel(ctx, d.UserID, d.Metric, d.Level)
		}
		g.bus.Publish(ctx, d)
	}
	return nil
}

func (g *GamifyService) GetState(ctx context.Context, user core.UserID) (core.UserState, error) {
	return g.storage.GetState(ctx, user)
}

func (g *GamifyService) Close() { g.bus.Close() }

type simpleRuleEngine struct{ rules []core.Rule }

func (s *simpleRuleEngine) Evaluate(ctx context.Context, state core.UserState, trigger core.Event) []core.Event {
	var out []core.Event
	for _, r := range s.rules {
		out = append(out, r.Evaluate(ctx, state, trigger)...)
	}
	return out
}
