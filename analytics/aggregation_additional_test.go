package analytics

import (
	"fmt"
	"testing"
	"time"

	"gamifykit/core"
)

func TestAggregationEngineWeeklyMonthly(t *testing.T) {
	metrics := NewComprehensiveMetrics()

	// Seed events across days
	base := time.Date(2024, 1, 3, 10, 0, 0, 0, time.UTC) // Wednesday
	evs := []core.Event{
		{Type: core.EventPointsAdded, UserID: "alice", Metric: core.MetricXP, Delta: 10, Time: base},
		{Type: core.EventPointsAdded, UserID: "bob", Metric: core.MetricXP, Delta: 20, Time: base.AddDate(0, 0, 1)}, // Thu
		{Type: core.EventBadgeAwarded, UserID: "alice", Badge: "onboarded", Time: base.AddDate(0, 0, 2)},            // Fri
	}
	for _, ev := range evs {
		metrics.OnEvent(ev)
	}

	ae := NewAggregationEngine(metrics, time.Hour)

	// Aggregate with fixed now within same week/month
	now := base
	if err := ae.aggregateWeekly(now); err != nil {
		t.Fatalf("weekly aggregate: %v", err)
	}
	if err := ae.aggregateMonthly(now); err != nil {
		t.Fatalf("monthly aggregate: %v", err)
	}

	year, week := now.UTC().ISOWeek()
	weekKey := fmt.Sprintf("%d-W%02d", year, week)
	weekly, ok := ae.GetAggregatedData(PeriodWeekly, weekKey)
	if !ok {
		t.Fatalf("missing weekly data")
	}
	if weekly.PointsAwarded != 30 || weekly.BadgesAwarded != 1 || weekly.ActiveUsers != 2 {
		t.Fatalf("unexpected weekly agg: %+v", weekly)
	}

	monthKey := now.UTC().Format("2006-01")
	monthly, ok := ae.GetAggregatedData(PeriodMonthly, monthKey)
	if !ok {
		t.Fatalf("missing monthly data")
	}
	if monthly.PointsAwarded != 30 || monthly.BadgesAwarded != 1 || monthly.ActiveUsers != 2 {
		t.Fatalf("unexpected monthly agg: %+v", monthly)
	}
}

func TestComprehensiveMetricsTopMetrics(t *testing.T) {
	metrics := NewComprehensiveMetrics()
	now := time.Now().UTC()
	metrics.OnEvent(core.Event{Type: core.EventPointsAdded, UserID: "u1", Metric: core.MetricXP, Delta: 10, Time: now})
	metrics.OnEvent(core.Event{Type: core.EventPointsAdded, UserID: "u1", Metric: core.MetricPoints, Delta: 20, Time: now})
	metrics.OnEvent(core.Event{Type: core.EventBadgeAwarded, UserID: "u1", Badge: "b1", Time: now})

	top := metrics.GetTopMetrics(5)
	totalPoints, ok := top["total_points_awarded"].(int64)
	if !ok || totalPoints != 30 {
		t.Fatalf("unexpected total points: %v", top["total_points_awarded"])
	}
	totalBadges, ok := top["total_badges_awarded"].(int64)
	if !ok || totalBadges != 1 {
		t.Fatalf("unexpected total badges: %v", top["total_badges_awarded"])
	}
}
