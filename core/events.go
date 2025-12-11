package core

import "time"

// EventType enumerates domain events.
type EventType string

const (
	EventPointsAdded         EventType = "points_added"
	EventBadgeAwarded        EventType = "badge_awarded"
	EventAchievementUnlocked EventType = "achievement_unlocked"
	EventLevelUp             EventType = "level_up"
)

// Event represents an immutable domain event.
type Event struct {
	Type     EventType      `json:"type"`
	Time     time.Time      `json:"time"`
	UserID   UserID         `json:"user_id"`
	Metric   Metric         `json:"metric,omitempty"`
	Delta    int64          `json:"delta,omitempty"`
	Total    int64          `json:"total,omitempty"`
	Badge    Badge          `json:"badge,omitempty"`
	Level    int64          `json:"level,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

func NewPointsAdded(user UserID, metric Metric, delta int64, total int64) Event {
	return Event{Type: EventPointsAdded, Time: time.Now().UTC(), UserID: user, Metric: metric, Delta: delta, Total: total}
}

func NewBadgeAwarded(user UserID, badge Badge) Event {
	return Event{Type: EventBadgeAwarded, Time: time.Now().UTC(), UserID: user, Badge: badge}
}

func NewLevelUp(user UserID, metric Metric, level int64) Event {
	return Event{Type: EventLevelUp, Time: time.Now().UTC(), UserID: user, Metric: metric, Level: level}
}
