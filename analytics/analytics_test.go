package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gamifykit/core"
)

func TestComprehensiveMetrics_OnEvent(t *testing.T) {
	metrics := NewComprehensiveMetrics()

	userID := core.UserID("user123")
	now := time.Now().UTC()

	// Test points awarded event
	event1 := core.Event{
		Type:   core.EventPointsAdded,
		UserID: userID,
		Time:   now,
		Metric: core.MetricXP,
		Delta:  100,
		Total:  100,
	}
	metrics.OnEvent(event1)

	// Test badge awarded event
	event2 := core.Event{
		Type:   core.EventBadgeAwarded,
		UserID: userID,
		Time:   now,
		Badge:  core.Badge("first_steps"),
	}
	metrics.OnEvent(event2)

	// Test level up event
	event3 := core.Event{
		Type:   core.EventLevelUp,
		UserID: userID,
		Time:   now,
		Metric: core.MetricXP,
		Level:  5,
	}
	metrics.OnEvent(event3)

	// Verify metrics
	dayKey := now.Format("2006-01-02")
	pointsByDay := metrics.GetPointsAwardedByDay(dayKey)
	badgesByDay := metrics.GetBadgesAwardedByDay(dayKey)
	activeUsers := metrics.GetDailyActiveUsers(dayKey)

	assert.Equal(t, int64(100), pointsByDay)
	assert.Equal(t, int64(1), badgesByDay)
	assert.Equal(t, 1, activeUsers)

	points, badges, levels := metrics.GetRealtimeStats()
	assert.Equal(t, int64(100), points)
	assert.Equal(t, int64(1), badges)
	assert.Equal(t, int64(1), levels)
}

func TestAggregationEngine(t *testing.T) {
	metrics := NewComprehensiveMetrics()
	aggregator := NewAggregationEngine(metrics, 1*time.Hour)

	// Add some test data
	userID := core.UserID("user123")
	now := time.Now().UTC()

	event := core.Event{
		Type:   core.EventPointsAdded,
		UserID: userID,
		Time:   now,
		Metric: core.MetricXP,
		Delta:  50,
		Total:  50,
	}
	metrics.OnEvent(event)

	// Force aggregation
	err := aggregator.AggregateNow()
	require.NoError(t, err)

	// Check daily aggregation
	dayKey := now.Format("2006-01-02")
	dailyData, exists := aggregator.GetAggregatedData(PeriodDaily, dayKey)
	require.True(t, exists)
	assert.Equal(t, PeriodDaily, dailyData.Period)
	assert.Equal(t, dayKey, dailyData.Key)
	assert.Equal(t, int64(50), dailyData.PointsAwarded)
}

func TestStreamPublisher(t *testing.T) {
	metrics := NewComprehensiveMetrics()
	publisher := NewStreamPublisher(metrics)

	// Create a test subscriber
	subscriber := NewInMemorySubscriber("test")

	// Subscribe
	publisher.Subscribe("test", subscriber)

	// Send an event
	userID := core.UserID("user123")
	event := core.Event{
		Type:   core.EventPointsAdded,
		UserID: userID,
		Time:   time.Now(),
		Metric: core.MetricXP,
		Delta:  25,
		Total:  25,
	}

	publisher.OnEvent(event)

	// Give it a moment to process
	time.Sleep(10 * time.Millisecond)

	// Check that subscriber received the event
	events := subscriber.GetEvents()
	require.Len(t, events, 1)
	assert.Equal(t, "points_awarded", events[0].Type)
	assert.Equal(t, userID, events[0].UserID)
	assert.Equal(t, int64(25), events[0].Points)

	// Unsubscribe
	publisher.Unsubscribe("test")
}

func TestConsoleExporter(t *testing.T) {
	exporter := NewConsoleExporter("[TEST]")

	data := &AggregatedData{
		Period:        PeriodDaily,
		Key:           "2024-01-01",
		ActiveUsers:   10,
		PointsAwarded: 1000,
		CreatedAt:     time.Now(),
	}

	// Export should not error (just prints to console)
	err := exporter.Export(context.Background(), data)
	assert.NoError(t, err)

	err = exporter.Flush(context.Background())
	assert.NoError(t, err)

	err = exporter.Close()
	assert.NoError(t, err)
}

func TestAnalyticsService(t *testing.T) {
	service := CreateAnalyticsServiceForTesting()

	// Get initial stats
	initialStats := service.GetRealtimeStats()
	assert.NotNil(t, initialStats)

	// Get dashboard data
	dashboard := service.GetDashboardData()
	assert.NotNil(t, dashboard)
	assert.Empty(t, dashboard.RecentEvents)

	// Force aggregation (should work without errors)
	err := service.ForceAggregation()
	assert.NoError(t, err)
}

func TestDashboardManager(t *testing.T) {
	metrics := NewComprehensiveMetrics()
	publisher := NewStreamPublisher(metrics)
	dashboard := NewDashboardManager(publisher, metrics, 5)

	// Add an event through the publisher
	event := core.Event{
		Type:   core.EventPointsAdded,
		UserID: core.UserID("user123"),
		Time:   time.Now(),
		Metric: core.MetricXP,
		Delta:  100,
		Total:  100,
	}

	publisher.OnEvent(event)

	// Give it a moment to process
	time.Sleep(10 * time.Millisecond)

	// Check dashboard data
	data := dashboard.GetDashboardData()
	assert.NotNil(t, data)
	assert.Len(t, data.RecentEvents, 1)
	assert.Equal(t, "points_awarded", data.RecentEvents[0].Type)
}

func BenchmarkComprehensiveMetrics(b *testing.B) {
	metrics := NewComprehensiveMetrics()

	event := core.Event{
		Type:   core.EventPointsAdded,
		UserID: core.UserID("user123"),
		Time:   time.Now(),
		Metric: core.MetricXP,
		Delta:  10,
		Total:  10,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metrics.OnEvent(event)
	}
}

func BenchmarkStreamPublisher(b *testing.B) {
	metrics := NewComprehensiveMetrics()
	publisher := NewStreamPublisher(metrics)
	subscriber := NewInMemorySubscriber("bench")

	publisher.Subscribe("bench", subscriber)

	event := core.Event{
		Type:   core.EventPointsAdded,
		UserID: core.UserID("user123"),
		Time:   time.Now(),
		Metric: core.MetricXP,
		Delta:  10,
		Total:  10,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		publisher.OnEvent(event)
	}
}
