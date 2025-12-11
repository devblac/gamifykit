package core

import "context"

// Rule determines whether given state and trigger event should emit derived events.
type Rule interface {
	Evaluate(ctx context.Context, state UserState, trigger Event) []Event
}

// LevelUpRule emits a level up when DefaultLevel increases.
type LevelUpRule struct{ Metric Metric }

func (r LevelUpRule) Evaluate(_ context.Context, state UserState, trigger Event) []Event {
	if trigger.Type != EventPointsAdded || trigger.Metric != r.Metric {
		return nil
	}
	total := state.Points[r.Metric]
	currentLevel := state.Levels[r.Metric]
	newLevel := DefaultLevel(total)
	if newLevel > currentLevel {
		return []Event{NewLevelUp(state.UserID, r.Metric, newLevel)}
	}
	return nil
}
