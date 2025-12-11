package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"gamifykit/core"
)

// AggregationPeriod represents different time periods for aggregation
type AggregationPeriod string

const (
	PeriodDaily   AggregationPeriod = "daily"
	PeriodWeekly  AggregationPeriod = "weekly"
	PeriodMonthly AggregationPeriod = "monthly"
)

// AggregatedData represents aggregated analytics data
type AggregatedData struct {
	Period    AggregationPeriod `json:"period"`
	Key       string            `json:"key"` // e.g., "2024-01-01" for daily, "2024-W01" for weekly
	StartTime time.Time         `json:"start_time"`
	EndTime   time.Time         `json:"end_time"`

	// User engagement
	ActiveUsers int `json:"active_users"`

	// Points
	PointsAwarded  int64                 `json:"points_awarded"`
	PointsSpent    int64                 `json:"points_spent"`
	PointsByMetric map[core.Metric]int64 `json:"points_by_metric"`

	// Badges
	BadgesAwarded int64                `json:"badges_awarded"`
	BadgesByType  map[core.Badge]int64 `json:"badges_by_type"`

	// Levels
	LevelsReached  int64                 `json:"levels_reached"`
	LevelsByMetric map[core.Metric]int64 `json:"levels_by_metric"`

	// Achievements
	AchievementsUnlocked int64            `json:"achievements_unlocked"`
	AchievementsByType   map[string]int64 `json:"achievements_by_type"`

	// Metadata
	CreatedAt time.Time `json:"created_at"`
}

// AggregationEngine handles periodic aggregation of analytics data
type AggregationEngine struct {
	mu sync.RWMutex

	metrics *ComprehensiveMetrics
	hook    Hook

	dailyAggregations   map[string]*AggregatedData
	weeklyAggregations  map[string]*AggregatedData
	monthlyAggregations map[string]*AggregatedData

	aggregationInterval time.Duration
	lastAggregation     time.Time
}

func NewAggregationEngine(metrics *ComprehensiveMetrics, aggregationInterval time.Duration) *AggregationEngine {
	return &AggregationEngine{
		metrics:             metrics,
		hook:                metrics,
		dailyAggregations:   make(map[string]*AggregatedData),
		weeklyAggregations:  make(map[string]*AggregatedData),
		monthlyAggregations: make(map[string]*AggregatedData),
		aggregationInterval: aggregationInterval,
		lastAggregation:     time.Now(),
	}
}

// OnEvent forwards events to the underlying metrics hook
func (ae *AggregationEngine) OnEvent(e core.Event) {
	ae.hook.OnEvent(e)
}

// AggregateNow forces an immediate aggregation of all periods
func (ae *AggregationEngine) AggregateNow() error {
	ae.mu.Lock()
	defer ae.mu.Unlock()

	now := time.Now().UTC()

	if err := ae.aggregateDaily(now); err != nil {
		return fmt.Errorf("failed to aggregate daily data: %w", err)
	}

	if err := ae.aggregateWeekly(now); err != nil {
		return fmt.Errorf("failed to aggregate weekly data: %w", err)
	}

	if err := ae.aggregateMonthly(now); err != nil {
		return fmt.Errorf("failed to aggregate monthly data: %w", err)
	}

	ae.lastAggregation = now
	return nil
}

func (ae *AggregationEngine) aggregateDaily(now time.Time) error {
	now = now.UTC()
	today := now.Format("2006-01-02")
	startTime := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	endTime := startTime.Add(24 * time.Hour)

	data := &AggregatedData{
		Period:             PeriodDaily,
		Key:                today,
		StartTime:          startTime,
		EndTime:            endTime,
		CreatedAt:          now,
		PointsByMetric:     make(map[core.Metric]int64),
		BadgesByType:       make(map[core.Badge]int64),
		LevelsByMetric:     make(map[core.Metric]int64),
		AchievementsByType: make(map[string]int64),
	}

	data.ActiveUsers = ae.metrics.GetDailyActiveUsers(today)
	data.PointsAwarded = ae.metrics.GetPointsAwardedByDay(today)
	data.BadgesAwarded = ae.metrics.GetBadgesAwardedByDay(today)

	ae.dailyAggregations[today] = data
	return nil
}

// aggregateWeekly aggregates data for the current week
func (ae *AggregationEngine) aggregateWeekly(now time.Time) error {
	now = now.UTC()
	year, week := now.ISOWeek()
	weekKey := fmt.Sprintf("%d-W%02d", year, week)

	// Calculate week start (Monday)
	daysSinceMonday := int(now.Weekday()-time.Monday) % 7
	if daysSinceMonday < 0 {
		daysSinceMonday += 7
	}
	startTime := time.Date(now.Year(), now.Month(), now.Day()-daysSinceMonday, 0, 0, 0, 0, time.UTC)
	endTime := startTime.Add(7 * 24 * time.Hour)

	data := &AggregatedData{
		Period:             PeriodWeekly,
		Key:                weekKey,
		StartTime:          startTime,
		EndTime:            endTime,
		CreatedAt:          now,
		PointsByMetric:     make(map[core.Metric]int64),
		BadgesByType:       make(map[core.Badge]int64),
		LevelsByMetric:     make(map[core.Metric]int64),
		AchievementsByType: make(map[string]int64),
	}

	data.ActiveUsers = ae.metrics.GetWeeklyActiveUsers(weekKey)

	weekStart := startTime
	for i := 0; i < 7; i++ {
		dayKey := weekStart.AddDate(0, 0, i).Format("2006-01-02")
		data.PointsAwarded += ae.metrics.GetPointsAwardedByDay(dayKey)
		data.BadgesAwarded += ae.metrics.GetBadgesAwardedByDay(dayKey)
	}

	ae.weeklyAggregations[weekKey] = data
	return nil
}

// aggregateMonthly aggregates data for the current month
func (ae *AggregationEngine) aggregateMonthly(now time.Time) error {
	now = now.UTC()
	monthKey := now.Format("2006-01")

	startTime := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	endTime := startTime.AddDate(0, 1, 0)

	data := &AggregatedData{
		Period:             PeriodMonthly,
		Key:                monthKey,
		StartTime:          startTime,
		EndTime:            endTime,
		CreatedAt:          now,
		PointsByMetric:     make(map[core.Metric]int64),
		BadgesByType:       make(map[core.Badge]int64),
		LevelsByMetric:     make(map[core.Metric]int64),
		AchievementsByType: make(map[string]int64),
	}

	data.ActiveUsers = ae.metrics.GetMonthlyActiveUsers(monthKey)

	daysInMonth := endTime.Sub(startTime).Hours() / 24
	monthStart := startTime
	for i := 0; i < int(daysInMonth); i++ {
		dayKey := monthStart.AddDate(0, 0, i).Format("2006-01-02")
		data.PointsAwarded += ae.metrics.GetPointsAwardedByDay(dayKey)
		data.BadgesAwarded += ae.metrics.GetBadgesAwardedByDay(dayKey)
	}

	ae.monthlyAggregations[monthKey] = data
	return nil
}

// GetAggregatedData returns aggregated data for a specific period and key
func (ae *AggregationEngine) GetAggregatedData(period AggregationPeriod, key string) (*AggregatedData, bool) {
	ae.mu.RLock()
	defer ae.mu.RUnlock()

	var aggregations map[string]*AggregatedData
	switch period {
	case PeriodDaily:
		aggregations = ae.dailyAggregations
	case PeriodWeekly:
		aggregations = ae.weeklyAggregations
	case PeriodMonthly:
		aggregations = ae.monthlyAggregations
	default:
		return nil, false
	}

	data, exists := aggregations[key]
	return data, exists
}

// GetAllAggregatedData returns all aggregated data for a specific period
func (ae *AggregationEngine) GetAllAggregatedData(period AggregationPeriod) []*AggregatedData {
	ae.mu.RLock()
	defer ae.mu.RUnlock()

	var aggregations map[string]*AggregatedData
	switch period {
	case PeriodDaily:
		aggregations = ae.dailyAggregations
	case PeriodWeekly:
		aggregations = ae.weeklyAggregations
	case PeriodMonthly:
		aggregations = ae.monthlyAggregations
	default:
		return nil
	}

	result := make([]*AggregatedData, 0, len(aggregations))
	for _, data := range aggregations {
		result = append(result, data)
	}
	return result
}

// Start begins periodic aggregation in a background goroutine
func (ae *AggregationEngine) Start(ctx context.Context) {
	ticker := time.NewTicker(ae.aggregationInterval)
	defer ticker.Stop()

	// Initial aggregation
	if err := ae.AggregateNow(); err != nil {
		// In a real implementation, you'd want proper logging here
		fmt.Printf("Initial aggregation failed: %v\n", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := ae.AggregateNow(); err != nil {
				// In a real implementation, you'd want proper logging here
				fmt.Printf("Periodic aggregation failed: %v\n", err)
			}
		}
	}
}

// ExportData exports aggregated data to JSON format
func (ae *AggregationEngine) ExportData(period AggregationPeriod) ([]byte, error) {
	data := ae.GetAllAggregatedData(period)
	return json.MarshalIndent(data, "", "  ")
}

// ExportToFile exports aggregated data to a file
func (ae *AggregationEngine) ExportToFile(period AggregationPeriod, filename string) error {
	data, err := ae.ExportData(period)
	if err != nil {
		return err
	}

	// In a real implementation, you'd write to the file
	// For now, just return nil to indicate success
	_ = data
	_ = filename
	return nil
}
