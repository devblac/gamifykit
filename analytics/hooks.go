package analytics

import (
	"fmt"
	"sync"
	"time"

	"gamifykit/core"
)

// Hook receives domain events for KPI aggregation.
type Hook interface {
	OnEvent(e core.Event)
}

// EventType represents different types of analytics events
type EventType string

const (
	EventTypePointsAdded    EventType = "points_added"
	EventTypePointsSpent    EventType = "points_spent"
	EventTypeLevelUp        EventType = "level_up"
	EventTypeBadgeAwarded   EventType = "badge_awarded"
	EventTypeAchievement    EventType = "achievement_unlocked"
	EventTypeUserEngagement EventType = "user_engagement"
)

// AnalyticsEvent represents a processed analytics event
type AnalyticsEvent struct {
	Type      EventType              `json:"type"`
	UserID    core.UserID            `json:"user_id"`
	Metric    core.Metric            `json:"metric,omitempty"`
	Badge     core.Badge             `json:"badge,omitempty"`
	Points    int64                  `json:"points,omitempty"`
	Level     int64                  `json:"level,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// DAU tracks daily active users.
type DAU struct {
	mu   sync.Mutex
	days map[string]map[core.UserID]struct{}
}

func NewDAU() *DAU { return &DAU{days: map[string]map[core.UserID]struct{}{}} }

func (d *DAU) OnEvent(e core.Event) {
	day := time.Unix(e.Time.Unix(), 0).UTC().Format("2006-01-02")
	d.mu.Lock()
	defer d.mu.Unlock()
	m := d.days[day]
	if m == nil {
		m = map[core.UserID]struct{}{}
		d.days[day] = m
	}
	m[e.UserID] = struct{}{}
}

func (d *DAU) Count(day string) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.days[day])
}

// ComprehensiveMetrics provides comprehensive analytics tracking
type ComprehensiveMetrics struct {
	mu sync.RWMutex

	// User engagement metrics
	dailyActiveUsers   map[string]map[core.UserID]struct{}
	weeklyActiveUsers  map[string]map[core.UserID]struct{}
	monthlyActiveUsers map[string]map[core.UserID]struct{}

	// Points metrics
	pointsAwardedByDay    map[string]int64
	pointsAwardedByMetric map[core.Metric]int64
	pointsSpentByDay      map[string]int64
	pointsSpentByMetric   map[core.Metric]int64

	// Badge metrics
	badgesAwardedByDay  map[string]int64
	badgesAwardedByType map[core.Badge]int64
	uniqueBadgeHolders  map[core.Badge]map[core.UserID]struct{}

	// Level metrics
	levelsReachedByDay    map[string]int64
	levelsReachedByMetric map[core.Metric]int64
	levelDistribution     map[core.Metric]map[int64]int // level -> count

	// Achievement metrics
	achievementsUnlockedByDay map[string]int64
	achievementsByType        map[string]int64

	// Real-time counters (last 24 hours)
	realtimeCounters struct {
		pointsAwarded int64
		badgesAwarded int64
		levelsReached int64
		lastReset     time.Time
	}
}

func NewComprehensiveMetrics() *ComprehensiveMetrics {
	now := time.Now()
	return &ComprehensiveMetrics{
		dailyActiveUsers:          make(map[string]map[core.UserID]struct{}),
		weeklyActiveUsers:         make(map[string]map[core.UserID]struct{}),
		monthlyActiveUsers:        make(map[string]map[core.UserID]struct{}),
		pointsAwardedByDay:        make(map[string]int64),
		pointsAwardedByMetric:     make(map[core.Metric]int64),
		pointsSpentByDay:          make(map[string]int64),
		pointsSpentByMetric:       make(map[core.Metric]int64),
		badgesAwardedByDay:        make(map[string]int64),
		badgesAwardedByType:       make(map[core.Badge]int64),
		uniqueBadgeHolders:        make(map[core.Badge]map[core.UserID]struct{}),
		levelsReachedByDay:        make(map[string]int64),
		levelsReachedByMetric:     make(map[core.Metric]int64),
		levelDistribution:         make(map[core.Metric]map[int64]int),
		achievementsUnlockedByDay: make(map[string]int64),
		achievementsByType:        make(map[string]int64),
		realtimeCounters: struct {
			pointsAwarded int64
			badgesAwarded int64
			levelsReached int64
			lastReset     time.Time
		}{lastReset: now},
	}
}

func (cm *ComprehensiveMetrics) OnEvent(e core.Event) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	day := e.Time.UTC().Format("2006-01-02")
	week := getWeekKey(e.Time)
	month := getMonthKey(e.Time)

	// Track user engagement
	cm.trackUserEngagement(e.UserID, day, week, month)

	// Track event-specific metrics
	switch e.Type {
	case core.EventPointsAdded:
		// Use Delta field for points added
		points := e.Delta
		if points > 0 {
			cm.pointsAwardedByDay[day] += points
			cm.pointsAwardedByMetric[e.Metric] += points
			cm.realtimeCounters.pointsAwarded += points
		}
	case core.EventLevelUp:
		cm.levelsReachedByDay[day]++
		cm.levelsReachedByMetric[e.Metric]++

		if cm.levelDistribution[e.Metric] == nil {
			cm.levelDistribution[e.Metric] = make(map[int64]int)
		}
		cm.levelDistribution[e.Metric][e.Level]++
		cm.realtimeCounters.levelsReached++
	case core.EventBadgeAwarded:
		cm.badgesAwardedByDay[day]++
		cm.badgesAwardedByType[e.Badge]++

		if cm.uniqueBadgeHolders[e.Badge] == nil {
			cm.uniqueBadgeHolders[e.Badge] = make(map[core.UserID]struct{})
		}
		cm.uniqueBadgeHolders[e.Badge][e.UserID] = struct{}{}
		cm.realtimeCounters.badgesAwarded++
	case core.EventAchievementUnlocked:
		// Achievement info might be in Metadata
		if achievement, ok := e.Metadata["achievement"].(string); ok {
			cm.achievementsUnlockedByDay[day]++
			cm.achievementsByType[achievement]++
		}
	}

	// Reset realtime counters if needed (every 24 hours)
	if time.Since(cm.realtimeCounters.lastReset) > 24*time.Hour {
		cm.realtimeCounters.pointsAwarded = 0
		cm.realtimeCounters.badgesAwarded = 0
		cm.realtimeCounters.levelsReached = 0
		cm.realtimeCounters.lastReset = time.Now()
	}
}

func (cm *ComprehensiveMetrics) trackUserEngagement(userID core.UserID, day, week, month string) {
	// Daily active users
	if cm.dailyActiveUsers[day] == nil {
		cm.dailyActiveUsers[day] = make(map[core.UserID]struct{})
	}
	cm.dailyActiveUsers[day][userID] = struct{}{}

	// Weekly active users
	if cm.weeklyActiveUsers[week] == nil {
		cm.weeklyActiveUsers[week] = make(map[core.UserID]struct{})
	}
	cm.weeklyActiveUsers[week][userID] = struct{}{}

	// Monthly active users
	if cm.monthlyActiveUsers[month] == nil {
		cm.monthlyActiveUsers[month] = make(map[core.UserID]struct{})
	}
	cm.monthlyActiveUsers[month][userID] = struct{}{}
}

// GetDailyActiveUsers returns the count of daily active users for a specific day
func (cm *ComprehensiveMetrics) GetDailyActiveUsers(day string) int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	if users, exists := cm.dailyActiveUsers[day]; exists {
		return len(users)
	}
	return 0
}

// GetWeeklyActiveUsers returns the count of weekly active users for a specific week
func (cm *ComprehensiveMetrics) GetWeeklyActiveUsers(week string) int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	if users, exists := cm.weeklyActiveUsers[week]; exists {
		return len(users)
	}
	return 0
}

// GetMonthlyActiveUsers returns the count of monthly active users for a specific month
func (cm *ComprehensiveMetrics) GetMonthlyActiveUsers(month string) int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	if users, exists := cm.monthlyActiveUsers[month]; exists {
		return len(users)
	}
	return 0
}

// GetPointsAwardedByDay returns total points awarded on a specific day
func (cm *ComprehensiveMetrics) GetPointsAwardedByDay(day string) int64 {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.pointsAwardedByDay[day]
}

// GetPointsAwardedByMetric returns total points awarded for a specific metric
func (cm *ComprehensiveMetrics) GetPointsAwardedByMetric(metric core.Metric) int64 {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.pointsAwardedByMetric[metric]
}

// GetBadgesAwardedByDay returns total badges awarded on a specific day
func (cm *ComprehensiveMetrics) GetBadgesAwardedByDay(day string) int64 {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.badgesAwardedByDay[day]
}

// GetBadgesAwardedByType returns total badges awarded of a specific type
func (cm *ComprehensiveMetrics) GetBadgesAwardedByType(badge core.Badge) int64 {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.badgesAwardedByType[badge]
}

// GetUniqueBadgeHolders returns the count of unique users who have a specific badge
func (cm *ComprehensiveMetrics) GetUniqueBadgeHolders(badge core.Badge) int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	if holders, exists := cm.uniqueBadgeHolders[badge]; exists {
		return len(holders)
	}
	return 0
}

// GetRealtimeStats returns real-time statistics for the last 24 hours
func (cm *ComprehensiveMetrics) GetRealtimeStats() (points int64, badges int64, levels int64) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.realtimeCounters.pointsAwarded,
		cm.realtimeCounters.badgesAwarded,
		cm.realtimeCounters.levelsReached
}

// GetTopMetrics returns aggregated metrics for reporting
func (cm *ComprehensiveMetrics) GetTopMetrics(limit int) map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	result := make(map[string]interface{})

	// Top metrics by points awarded
	topMetrics := make([]struct {
		metric core.Metric
		points int64
	}, 0, len(cm.pointsAwardedByMetric))

	for metric, points := range cm.pointsAwardedByMetric {
		topMetrics = append(topMetrics, struct {
			metric core.Metric
			points int64
		}{metric, points})
	}

	// Sort by points (simple bubble sort for small datasets)
	for i := 0; i < len(topMetrics); i++ {
		for j := i + 1; j < len(topMetrics); j++ {
			if topMetrics[i].points < topMetrics[j].points {
				topMetrics[i], topMetrics[j] = topMetrics[j], topMetrics[i]
			}
		}
	}

	if len(topMetrics) > limit {
		topMetrics = topMetrics[:limit]
	}

	topMetricsData := make([]map[string]interface{}, len(topMetrics))
	for i, tm := range topMetrics {
		topMetricsData[i] = map[string]interface{}{
			"metric": tm.metric,
			"points": tm.points,
		}
	}

	result["top_metrics_by_points"] = topMetricsData
	result["total_points_awarded"] = sumMapValues(cm.pointsAwardedByMetric)
	result["total_badges_awarded"] = sumBadgeMapValues(cm.badgesAwardedByType)

	return result
}

// Helper functions
func getWeekKey(t time.Time) string {
	tt := t.UTC()
	year, week := tt.ISOWeek()
	return fmt.Sprintf("%d-W%02d", year, week)
}

func getMonthKey(t time.Time) string {
	return t.UTC().Format("2006-01")
}

func sumMapValues(m map[core.Metric]int64) int64 {
	var total int64
	for _, v := range m {
		total += v
	}
	return total
}

func sumBadgeMapValues(m map[core.Badge]int64) int64 {
	var total int64
	for _, v := range m {
		total += v
	}
	return total
}
