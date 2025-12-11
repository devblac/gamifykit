package analytics

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"time"

	"gamifykit/core"
)

// StreamEvent represents a real-time analytics event for streaming
type StreamEvent struct {
	Type      string                 `json:"type"`
	UserID    core.UserID            `json:"user_id"`
	Metric    core.Metric            `json:"metric,omitempty"`
	Badge     core.Badge             `json:"badge,omitempty"`
	Points    int64                  `json:"points,omitempty"`
	Level     int64                  `json:"level,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// StreamSubscriber represents a subscriber to real-time analytics events
type StreamSubscriber interface {
	OnStreamEvent(event *StreamEvent)
	Close() error
}

// StreamPublisher manages real-time analytics streaming
type StreamPublisher struct {
	mu          sync.RWMutex
	subscribers map[string]StreamSubscriber
	metrics     *ComprehensiveMetrics
}

func NewStreamPublisher(metrics *ComprehensiveMetrics) *StreamPublisher {
	return &StreamPublisher{
		subscribers: make(map[string]StreamSubscriber),
		metrics:     metrics,
	}
}

// Subscribe adds a subscriber to receive real-time events
func (sp *StreamPublisher) Subscribe(id string, subscriber StreamSubscriber) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.subscribers[id] = subscriber
}

// Unsubscribe removes a subscriber
func (sp *StreamPublisher) Unsubscribe(id string) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	if subscriber, exists := sp.subscribers[id]; exists {
		if err := subscriber.Close(); err != nil {
			// Log error but continue with cleanup
			// In production, use proper logging
		}
		delete(sp.subscribers, id)
	}
}

// PublishEvent publishes an event to all subscribers
func (sp *StreamPublisher) PublishEvent(event *StreamEvent) {
	sp.mu.RLock()
	subscribers := make([]StreamSubscriber, 0, len(sp.subscribers))
	for _, subscriber := range sp.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	sp.mu.RUnlock()

	for _, subscriber := range subscribers {
		func(sub StreamSubscriber) {
			defer func() {
				if r := recover(); r != nil {
					// swallow subscriber panic to keep publisher alive
				}
			}()
			sub.OnStreamEvent(event)
		}(subscriber)
	}
}

// OnEvent processes gamification events and publishes them as stream events
func (sp *StreamPublisher) OnEvent(e core.Event) {
	// First, let the metrics system process the event
	sp.metrics.OnEvent(e)

	// Convert to stream event
	streamEvent := sp.convertToStreamEvent(e)

	// Publish to subscribers
	sp.PublishEvent(streamEvent)
}

func (sp *StreamPublisher) convertToStreamEvent(e core.Event) *StreamEvent {
	event := &StreamEvent{
		Type:      string(e.Type),
		UserID:    e.UserID,
		Timestamp: e.Time,
		Metadata:  make(map[string]interface{}),
	}

	// Extract event-specific data
	switch e.Type {
	case core.EventPointsAdded:
		event.Type = "points_awarded"
		event.Points = e.Delta
		event.Metric = e.Metric
	case core.EventLevelUp:
		event.Type = "level_reached"
		event.Level = e.Level
		event.Metric = e.Metric
	case core.EventBadgeAwarded:
		event.Type = "badge_awarded"
		event.Badge = e.Badge
	case core.EventAchievementUnlocked:
		event.Type = "achievement_unlocked"
		if achievement, ok := e.Metadata["achievement"].(string); ok {
			event.Metadata["achievement"] = achievement
		}
	}

	return event
}

// GetRealtimeStats returns current real-time statistics
func (sp *StreamPublisher) GetRealtimeStats() map[string]interface{} {
	points, badges, levels := sp.metrics.GetRealtimeStats()

	return map[string]interface{}{
		"points_awarded_24h": points,
		"badges_awarded_24h": badges,
		"levels_reached_24h": levels,
		"active_subscribers": len(sp.subscribers),
		"timestamp":          time.Now(),
	}
}

// WebSocketSubscriber streams events to WebSocket clients
type WebSocketSubscriber struct {
	id        string
	sendChan  chan *StreamEvent
	closeChan chan struct{}
}

func NewWebSocketSubscriber(id string, bufferSize int) *WebSocketSubscriber {
	return &WebSocketSubscriber{
		id:        id,
		sendChan:  make(chan *StreamEvent, bufferSize),
		closeChan: make(chan struct{}),
	}
}

func (ws *WebSocketSubscriber) OnStreamEvent(event *StreamEvent) {
	select {
	case ws.sendChan <- event:
		// Event sent successfully
	case <-ws.closeChan:
		// Subscriber is closing
	default:
		// Channel is full, drop the event
	}
}

// ReadEvent reads an event from the subscriber channel
func (ws *WebSocketSubscriber) ReadEvent(ctx context.Context) (*StreamEvent, error) {
	select {
	case event := <-ws.sendChan:
		return event, nil
	case <-ws.closeChan:
		return nil, io.EOF
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (ws *WebSocketSubscriber) Close() error {
	select {
	case <-ws.closeChan:
		// Already closed
	default:
		close(ws.closeChan)
	}
	return nil
}

// InMemorySubscriber stores events in memory for testing/debugging
type InMemorySubscriber struct {
	id     string
	events []*StreamEvent
	mu     sync.RWMutex
}

func NewInMemorySubscriber(id string) *InMemorySubscriber {
	return &InMemorySubscriber{
		id:     id,
		events: make([]*StreamEvent, 0),
	}
}

func (ims *InMemorySubscriber) OnStreamEvent(event *StreamEvent) {
	ims.mu.Lock()
	defer ims.mu.Unlock()
	ims.events = append(ims.events, event)
}

func (ims *InMemorySubscriber) GetEvents() []*StreamEvent {
	ims.mu.RLock()
	defer ims.mu.RUnlock()
	result := make([]*StreamEvent, len(ims.events))
	copy(result, ims.events)
	return result
}

func (ims *InMemorySubscriber) ClearEvents() {
	ims.mu.Lock()
	defer ims.mu.Unlock()
	ims.events = ims.events[:0]
}

func (ims *InMemorySubscriber) Close() error {
	return nil
}

// DashboardData represents data for live dashboards
type DashboardData struct {
	RealtimeStats map[string]interface{} `json:"realtime_stats"`
	TopMetrics    map[string]interface{} `json:"top_metrics"`
	RecentEvents  []*StreamEvent         `json:"recent_events"`
	Timestamp     time.Time              `json:"timestamp"`
}

// DashboardManager manages dashboard data and updates
type DashboardManager struct {
	publisher    *StreamPublisher
	metrics      *ComprehensiveMetrics
	recentEvents []*StreamEvent
	maxEvents    int
	mu           sync.RWMutex
}

// OnStreamEvent implements StreamSubscriber interface
func (dm *DashboardManager) OnStreamEvent(event *StreamEvent) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.recentEvents = append(dm.recentEvents, event)
	if len(dm.recentEvents) > dm.maxEvents {
		dm.recentEvents = dm.recentEvents[1:] // Remove oldest
	}
}

// Close implements StreamSubscriber interface
func (dm *DashboardManager) Close() error {
	return nil
}

func NewDashboardManager(publisher *StreamPublisher, metrics *ComprehensiveMetrics, maxEvents int) *DashboardManager {
	dm := &DashboardManager{
		publisher:    publisher,
		metrics:      metrics,
		recentEvents: make([]*StreamEvent, 0, maxEvents),
		maxEvents:    maxEvents,
	}

	// Subscribe to events to maintain recent events list
	publisher.Subscribe("dashboard", dm)

	return dm
}

// GetDashboardData returns current dashboard data
func (dm *DashboardManager) GetDashboardData() *DashboardData {
	dm.mu.RLock()
	recentEvents := make([]*StreamEvent, len(dm.recentEvents))
	copy(recentEvents, dm.recentEvents)
	dm.mu.RUnlock()

	return &DashboardData{
		RealtimeStats: dm.publisher.GetRealtimeStats(),
		TopMetrics:    dm.metrics.GetTopMetrics(10),
		RecentEvents:  recentEvents,
		Timestamp:     time.Now(),
	}
}

// GetDashboardDataJSON returns dashboard data as JSON
func (dm *DashboardManager) GetDashboardDataJSON() ([]byte, error) {
	data := dm.GetDashboardData()
	return json.Marshal(data)
}
