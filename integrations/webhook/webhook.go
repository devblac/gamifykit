package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"gamifykit/core"
)

// Sink posts domain events to configured HTTP endpoints.
// It is synchronous for determinism; keep handlers fast or wrap with buffering if needed.
type Sink struct {
	client    *http.Client
	endpoints []string
}

// Option configures a Sink.
type Option func(*Sink)

// WithClient overrides the HTTP client (defaults to 2s timeout).
func WithClient(c *http.Client) Option {
	return func(s *Sink) {
		if c != nil {
			s.client = c
		}
	}
}

// New creates a webhook sink.
func New(endpoints []string, opts ...Option) *Sink {
	s := &Sink{
		client: &http.Client{Timeout: 2 * time.Second},
	}
	for _, opt := range opts {
		opt(s)
	}
	s.endpoints = append([]string{}, endpoints...)
	return s
}

// OnEvent posts the event JSON to all endpoints; errors are ignored for now (MVP).
func (s *Sink) OnEvent(e core.Event) {
	if len(s.endpoints) == 0 {
		return
	}
	body, err := json.Marshal(e)
	if err != nil {
		return
	}
	for _, ep := range s.endpoints {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, ep, bytes.NewReader(body))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		_, _ = s.client.Do(req)
	}
}

