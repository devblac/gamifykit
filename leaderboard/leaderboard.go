package leaderboard

import "gamifykit/core"

// Entry represents a score entry.
type Entry struct {
	User  core.UserID
	Score int64
}

// Board abstracts leaderboard operations.
type Board interface {
	Update(user core.UserID, score int64)
	Remove(user core.UserID)
	TopN(n int) []Entry
	Get(user core.UserID) (Entry, bool)
}
