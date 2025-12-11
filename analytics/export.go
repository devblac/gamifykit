package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Exporter defines the interface for exporting analytics data
type Exporter interface {
	Export(ctx context.Context, data *AggregatedData) error
	Flush(ctx context.Context) error
	Close() error
}

// HTTPExporter exports data to external HTTP endpoints
type HTTPExporter struct {
	endpoint   string
	apiKey     string
	httpClient *http.Client
	buffer     []*AggregatedData
	batchSize  int
}

func NewHTTPExporter(endpoint, apiKey string, batchSize int) *HTTPExporter {
	return &HTTPExporter{
		endpoint: endpoint,
		apiKey:   apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		buffer:    make([]*AggregatedData, 0, batchSize),
		batchSize: batchSize,
	}
}

func (e *HTTPExporter) Export(ctx context.Context, data *AggregatedData) error {
	e.buffer = append(e.buffer, data)

	if len(e.buffer) >= e.batchSize {
		return e.Flush(ctx)
	}

	return nil
}

func (e *HTTPExporter) Flush(ctx context.Context) error {
	if len(e.buffer) == 0 {
		return nil
	}

	payload, err := json.Marshal(e.buffer)
	if err != nil {
		return fmt.Errorf("failed to marshal analytics data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send analytics data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("analytics export failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Clear buffer on successful export
	e.buffer = e.buffer[:0]
	return nil
}

func (e *HTTPExporter) Close() error {
	// Flush any remaining data
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.Flush(ctx); err != nil {
		return err
	}

	return nil
}

// SegmentExporter exports data to Segment analytics
type SegmentExporter struct {
	writeKey   string
	httpClient *http.Client
}

type segmentEvent struct {
	UserID     string                 `json:"userId"`
	Event      string                 `json:"event"`
	Timestamp  time.Time              `json:"timestamp"`
	Properties map[string]interface{} `json:"properties"`
}

func NewSegmentExporter(writeKey string) *SegmentExporter {
	return &SegmentExporter{
		writeKey: writeKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (e *SegmentExporter) Export(ctx context.Context, data *AggregatedData) error {
	// Convert aggregated data to Segment events
	events := e.convertToSegmentEvents(data)

	for _, event := range events {
		if err := e.sendEvent(ctx, event); err != nil {
			return fmt.Errorf("failed to send segment event: %w", err)
		}
	}

	return nil
}

func (e *SegmentExporter) convertToSegmentEvents(data *AggregatedData) []segmentEvent {
	events := []segmentEvent{}

	// Create events for key metrics
	if data.ActiveUsers > 0 {
		events = append(events, segmentEvent{
			UserID:    "system", // System-level event
			Event:     "gamification_active_users",
			Timestamp: data.CreatedAt,
			Properties: map[string]interface{}{
				"period":       data.Period,
				"period_key":   data.Key,
				"active_users": data.ActiveUsers,
			},
		})
	}

	if data.PointsAwarded > 0 {
		events = append(events, segmentEvent{
			UserID:    "system",
			Event:     "gamification_points_awarded",
			Timestamp: data.CreatedAt,
			Properties: map[string]interface{}{
				"period":           data.Period,
				"period_key":       data.Key,
				"points_awarded":   data.PointsAwarded,
				"points_by_metric": data.PointsByMetric,
			},
		})
	}

	if data.BadgesAwarded > 0 {
		events = append(events, segmentEvent{
			UserID:    "system",
			Event:     "gamification_badges_awarded",
			Timestamp: data.CreatedAt,
			Properties: map[string]interface{}{
				"period":         data.Period,
				"period_key":     data.Key,
				"badges_awarded": data.BadgesAwarded,
				"badges_by_type": data.BadgesByType,
			},
		})
	}

	return events
}

func (e *SegmentExporter) sendEvent(ctx context.Context, event segmentEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.segment.io/v1/track", bytes.NewReader(payload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(e.writeKey, "")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("segment API returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (e *SegmentExporter) Flush(ctx context.Context) error {
	// Segment events are sent immediately, so no batching to flush
	return nil
}

func (e *SegmentExporter) Close() error {
	return nil
}

// ConsoleExporter exports data to console (for debugging)
type ConsoleExporter struct {
	prefix string
}

func NewConsoleExporter(prefix string) *ConsoleExporter {
	return &ConsoleExporter{prefix: prefix}
}

func (e *ConsoleExporter) Export(ctx context.Context, data *AggregatedData) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	fmt.Printf("%s Analytics Export:\n%s\n", e.prefix, string(jsonData))
	return nil
}

func (e *ConsoleExporter) Flush(ctx context.Context) error {
	return nil
}

func (e *ConsoleExporter) Close() error {
	return nil
}

// MultiExporter combines multiple exporters
type MultiExporter struct {
	exporters []Exporter
}

func NewMultiExporter(exporters ...Exporter) *MultiExporter {
	return &MultiExporter{exporters: exporters}
}

func (e *MultiExporter) Export(ctx context.Context, data *AggregatedData) error {
	for _, exporter := range e.exporters {
		if err := exporter.Export(ctx, data); err != nil {
			// Log error but continue with other exporters
			fmt.Printf("Export error: %v\n", err)
		}
	}
	return nil
}

func (e *MultiExporter) Flush(ctx context.Context) error {
	for _, exporter := range e.exporters {
		if err := exporter.Flush(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (e *MultiExporter) Close() error {
	var lastErr error
	for _, exporter := range e.exporters {
		if err := exporter.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// ExportManager manages multiple exporters and handles data distribution
type ExportManager struct {
	exporters []Exporter
}

func NewExportManager(exporters ...Exporter) *ExportManager {
	return &ExportManager{exporters: exporters}
}

// ExportData distributes data to all configured exporters
func (em *ExportManager) ExportData(ctx context.Context, data []*AggregatedData) error {
	for _, aggregatedData := range data {
		for _, exporter := range em.exporters {
			if err := exporter.Export(ctx, aggregatedData); err != nil {
				return fmt.Errorf("export failed for %T: %w", exporter, err)
			}
		}
	}

	// Flush all exporters
	return em.Flush(ctx)
}

// Flush flushes all exporters
func (em *ExportManager) Flush(ctx context.Context) error {
	for _, exporter := range em.exporters {
		if err := exporter.Flush(ctx); err != nil {
			return fmt.Errorf("flush failed for %T: %w", exporter, err)
		}
	}
	return nil
}

// Close closes all exporters
func (em *ExportManager) Close() error {
	var lastErr error
	for _, exporter := range em.exporters {
		if err := exporter.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}
