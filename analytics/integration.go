package analytics

import (
	"context"
	"fmt"
	"time"

	"gamifykit/core"
	"gamifykit/engine"
)

// AnalyticsService provides a complete analytics solution integrated with gamification
type AnalyticsService struct {
	metrics    *ComprehensiveMetrics
	aggregator *AggregationEngine
	publisher  *StreamPublisher
	dashboard  *DashboardManager
	exporter   *ExportManager
}

// NewAnalyticsService creates a fully configured analytics service
func NewAnalyticsService() *AnalyticsService {
	// Create core metrics
	metrics := NewComprehensiveMetrics()

	// Create aggregation engine (aggregate every hour)
	aggregator := NewAggregationEngine(metrics, 1*time.Hour)

	// Create streaming publisher
	publisher := NewStreamPublisher(metrics)

	// Create dashboard manager
	dashboard := NewDashboardManager(publisher, metrics, 100)

	// Create exporters (console for demo, can add HTTP/Segment exporters)
	exporters := []Exporter{
		NewConsoleExporter("[ANALYTICS]"),
	}
	exporter := NewExportManager(exporters...)

	return &AnalyticsService{
		metrics:    metrics,
		aggregator: aggregator,
		publisher:  publisher,
		dashboard:  dashboard,
		exporter:   exporter,
	}
}

// GetHook returns a hook that can be registered with the gamification engine
func (as *AnalyticsService) GetHook() Hook {
	// Return the publisher which forwards to metrics
	return as.publisher
}

// Start begins background analytics processing
func (as *AnalyticsService) Start(ctx context.Context) {
	// Start aggregation in background
	go as.aggregator.Start(ctx)

	// Start periodic export in background
	go as.startPeriodicExport(ctx)
}

// startPeriodicExport periodically exports aggregated data
func (as *AnalyticsService) startPeriodicExport(ctx context.Context) {
	ticker := time.NewTicker(6 * time.Hour) // Export every 6 hours
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Export daily aggregations
			dailyData := as.aggregator.GetAllAggregatedData(PeriodDaily)
			if err := as.exporter.ExportData(ctx, dailyData); err != nil {
				// In production, use proper logging
				fmt.Printf("Export error: %v\n", err)
			}
		}
	}
}

// GetRealtimeStats returns current real-time statistics
func (as *AnalyticsService) GetRealtimeStats() map[string]interface{} {
	return as.publisher.GetRealtimeStats()
}

// GetDashboardData returns data for live dashboards
func (as *AnalyticsService) GetDashboardData() *DashboardData {
	return as.dashboard.GetDashboardData()
}

// ForceAggregation triggers immediate aggregation (useful for testing)
func (as *AnalyticsService) ForceAggregation() error {
	return as.aggregator.AggregateNow()
}

// SubscribeToRealtime adds a subscriber for real-time events
func (as *AnalyticsService) SubscribeToRealtime(id string, subscriber StreamSubscriber) {
	as.publisher.Subscribe(id, subscriber)
}

// UnsubscribeFromRealtime removes a real-time subscriber
func (as *AnalyticsService) UnsubscribeFromRealtime(id string) {
	as.publisher.Unsubscribe(id)
}

// Example integration with gamification engine
func ExampleIntegration() {
	// Create analytics service
	analytics := NewAnalyticsService()

	// Create gamification service with analytics integration
	svc := engine.NewGamifyService(
		/* storage */ nil, // Add your storage
		/* bus */ nil, // Add your event bus
		/* rules */ nil, // Add your rule engine
	)

	// Subscribe analytics hook to all events
	analyticsHook := analytics.GetHook()
	svc.Subscribe(core.EventPointsAdded, func(ctx context.Context, e core.Event) {
		analyticsHook.OnEvent(e)
	})
	svc.Subscribe(core.EventBadgeAwarded, func(ctx context.Context, e core.Event) {
		analyticsHook.OnEvent(e)
	})
	svc.Subscribe(core.EventLevelUp, func(ctx context.Context, e core.Event) {
		analyticsHook.OnEvent(e)
	})
	svc.Subscribe(core.EventAchievementUnlocked, func(ctx context.Context, e core.Event) {
		analyticsHook.OnEvent(e)
	})

	// Start analytics in background
	ctx := context.Background()
	analytics.Start(ctx)

	// Example: Trigger some gamification events
	userID := core.UserID("user123")

	// Award points
	if _, err := svc.AddPoints(ctx, userID, core.MetricXP, 100); err != nil {
		fmt.Printf("Failed to award points: %v\n", err)
	}

	// Award badge
	if err := svc.AwardBadge(ctx, userID, core.Badge("first_steps")); err != nil {
		fmt.Printf("Failed to award badge: %v\n", err)
	}

	// Get real-time stats
	stats := analytics.GetRealtimeStats()
	fmt.Printf("Real-time stats: %+v\n", stats)

	// Get dashboard data
	dashboard := analytics.GetDashboardData()
	fmt.Printf("Dashboard has %d recent events\n", len(dashboard.RecentEvents))
}

// AdvancedIntegrationExample shows how to set up analytics with external exports
func AdvancedIntegrationExample() {
	// Create analytics service
	analytics := NewAnalyticsService()

	// Add external exporters
	externalExporters := []Exporter{
		NewHTTPExporter("https://analytics.example.com/webhook", "api-key-123", 10),
		NewSegmentExporter("segment-write-key"),
	}

	// Replace the export manager with one that includes external exports
	analytics.exporter = NewExportManager(append([]Exporter{
		NewConsoleExporter("[ANALYTICS]"),
	}, externalExporters...)...)

	// The rest of the setup is the same as the basic example
	svc := engine.NewGamifyService(nil, nil, nil)

	// Subscribe analytics hook to all events
	analyticsHook := analytics.GetHook()
	svc.Subscribe(core.EventPointsAdded, func(ctx context.Context, e core.Event) {
		analyticsHook.OnEvent(e)
	})
	svc.Subscribe(core.EventBadgeAwarded, func(ctx context.Context, e core.Event) {
		analyticsHook.OnEvent(e)
	})
	svc.Subscribe(core.EventLevelUp, func(ctx context.Context, e core.Event) {
		analyticsHook.OnEvent(e)
	})
	svc.Subscribe(core.EventAchievementUnlocked, func(ctx context.Context, e core.Event) {
		analyticsHook.OnEvent(e)
	})

	ctx := context.Background()
	analytics.Start(ctx)

	// Analytics will now export to external services every 6 hours
}

// CreateAnalyticsServiceForTesting creates a minimal analytics setup for testing
func CreateAnalyticsServiceForTesting() *AnalyticsService {
	metrics := NewComprehensiveMetrics()
	aggregator := NewAggregationEngine(metrics, 1*time.Hour)
	publisher := NewStreamPublisher(metrics)
	dashboard := NewDashboardManager(publisher, metrics, 10)

	// Only console exporter for testing
	exporter := NewExportManager(NewConsoleExporter("[TEST]"))

	return &AnalyticsService{
		metrics:    metrics,
		aggregator: aggregator,
		publisher:  publisher,
		dashboard:  dashboard,
		exporter:   exporter,
	}
}

// AnalyticsConfig holds configuration for analytics services
type AnalyticsConfig struct {
	AggregationInterval time.Duration    `json:"aggregation_interval"`
	MaxRecentEvents     int              `json:"max_recent_events"`
	ExportInterval      time.Duration    `json:"export_interval"`
	EnableStreaming     bool             `json:"enable_streaming"`
	Exporters           []ExporterConfig `json:"exporters"`
}

// ExporterConfig holds configuration for individual exporters
type ExporterConfig struct {
	Type       string            `json:"type"` // "http", "segment", "console"
	Endpoint   string            `json:"endpoint,omitempty"`
	APIKey     string            `json:"api_key,omitempty"`
	BatchSize  int               `json:"batch_size,omitempty"`
	Properties map[string]string `json:"properties,omitempty"`
}

// NewAnalyticsServiceWithConfig creates analytics service with custom configuration
func NewAnalyticsServiceWithConfig(config *AnalyticsConfig) *AnalyticsService {
	metrics := NewComprehensiveMetrics()
	aggregator := NewAggregationEngine(metrics, config.AggregationInterval)
	publisher := NewStreamPublisher(metrics)
	dashboard := NewDashboardManager(publisher, metrics, config.MaxRecentEvents)

	// Create exporters from config
	exporters := []Exporter{NewConsoleExporter("[ANALYTICS]")}
	for _, expConfig := range config.Exporters {
		switch expConfig.Type {
		case "http":
			exporter := NewHTTPExporter(expConfig.Endpoint, expConfig.APIKey, expConfig.BatchSize)
			if expConfig.BatchSize == 0 {
				expConfig.BatchSize = 10 // default
			}
			exporters = append(exporters, exporter)
		case "segment":
			if expConfig.APIKey != "" {
				exporters = append(exporters, NewSegmentExporter(expConfig.APIKey))
			}
		}
	}

	exporter := NewExportManager(exporters...)

	return &AnalyticsService{
		metrics:    metrics,
		aggregator: aggregator,
		publisher:  publisher,
		dashboard:  dashboard,
		exporter:   exporter,
	}
}
