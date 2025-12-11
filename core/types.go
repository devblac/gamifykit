package core

import (
	"errors"
	"math"
	"strings"
	"time"
)

// UserID uniquely identifies a user in the gamification domain.
type UserID string

// Metric represents a points counter namespace such as XP or generic POINTS.
type Metric string

const (
	MetricXP     Metric = "xp"
	MetricPoints Metric = "points"
)

// Badge represents a named badge identifier.
type Badge string

// UserState is an immutable snapshot of a user's gamification state.
// Implementations should return deep copies to maintain immutability guarantees.
type UserState struct {
	UserID  UserID             `json:"user_id"`
	Points  map[Metric]int64   `json:"points"`
	Badges  map[Badge]struct{} `json:"badges"`
	Levels  map[Metric]int64   `json:"levels"`
	Updated time.Time          `json:"updated"`
}

// Clone returns a deep copy of the state to uphold immutability.
func (s UserState) Clone() UserState {
	cp := UserState{
		UserID:  s.UserID,
		Points:  make(map[Metric]int64, len(s.Points)),
		Badges:  make(map[Badge]struct{}, len(s.Badges)),
		Levels:  make(map[Metric]int64, len(s.Levels)),
		Updated: s.Updated,
	}
	for k, v := range s.Points {
		cp.Points[k] = v
	}
	for k := range s.Badges {
		cp.Badges[k] = struct{}{}
	}
	for k, v := range s.Levels {
		cp.Levels[k] = v
	}
	return cp
}

// AddSafe adds delta to base ensuring no signed overflow occurs.
func AddSafe(base int64, delta int64) (int64, error) {
	if (delta > 0 && base > math.MaxInt64-delta) || (delta < 0 && base < math.MinInt64-delta) {
		return 0, errors.New("integer overflow in AddSafe")
	}
	return base + delta, nil
}

// NormalizeUserID trims and lowercases user identifiers.
func NormalizeUserID(id UserID) (UserID, error) {
	s := strings.TrimSpace(string(id))
	if s == "" {
		return "", errors.New("empty user id")
	}
	return UserID(strings.ToLower(s)), nil
}

// ValidateBadgeID ensures non-empty badge id with simple charset check.
func ValidateBadgeID(b Badge) error {
	s := strings.TrimSpace(string(b))
	if s == "" {
		return errors.New("empty badge id")
	}
	// simple check: alnum, dash, underscore
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			continue
		}
		return errors.New("invalid badge id")
	}
	return nil
}

// DefaultLevel computes a level from total XP using a sublinear curve.
// level = floor(sqrt(xp)/10) + 1, ensuring at least 1.
func DefaultLevel(totalXP int64) int64 {
	if totalXP <= 0 {
		return 1
	}
	lvl := int64(math.Floor(math.Sqrt(float64(totalXP))/10.0)) + 1
	if lvl < 1 {
		return 1
	}
	return lvl
}
