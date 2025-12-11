package engine

import (
	"context"
	"gamifykit/core"
)

// Storage abstracts persistence for gamification state.
type Storage interface {
	AddPoints(ctx context.Context, user core.UserID, metric core.Metric, delta int64) (newTotal int64, err error)
	AwardBadge(ctx context.Context, user core.UserID, badge core.Badge) error
	GetState(ctx context.Context, user core.UserID) (core.UserState, error)
	SetLevel(ctx context.Context, user core.UserID, metric core.Metric, level int64) error
}

// RuleEngine evaluates rules and emits derived events.
type RuleEngine interface {
	Evaluate(ctx context.Context, state core.UserState, trigger core.Event) []core.Event
}
