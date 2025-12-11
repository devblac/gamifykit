package httpapi

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	wsadapter "gamifykit/adapters/websocket"
	"gamifykit/core"
	"gamifykit/engine"
	"gamifykit/realtime"
)

// Options configures the HTTP API surface.
type Options struct {
	// PathPrefix, if set, is prepended to all routes (e.g., "/api").
	PathPrefix string
	// AllowCORSOrigin, if non-empty, enables basic CORS with the given origin (use "*" for any).
	AllowCORSOrigin string
	// APIKeys, if non-empty, enables static API key auth via Authorization: Bearer or X-API-Key.
	APIKeys []string
	// RateLimitEnabled toggles rate limiting.
	RateLimitEnabled bool
	// RateLimitRPM is the allowed requests per minute per client key.
	RateLimitRPM int
	// RateLimitBurst defines burst capacity.
	RateLimitBurst int
}

// NewMux builds an http.Handler exposing a minimal Gamify REST API and WebSocket stream.
// Routes:
//   - POST {prefix}/users/{id}/points?metric=xp&delta=50
//   - POST {prefix}/users/{id}/badges/{badge}
//   - GET  {prefix}/users/{id}
//   - GET  {prefix}/healthz
//   - WS   {prefix}/ws
func NewMux(svc *engine.GamifyService, hub *realtime.Hub, opts Options) http.Handler {
	mux := http.NewServeMux()

	// health
	mux.HandleFunc(withPrefix(opts.PathPrefix, "/healthz"), func(w http.ResponseWriter, r *http.Request) {
		healthCheck(w, r, svc)
	})

	// WebSocket events
	if hub != nil {
		mux.Handle(withPrefix(opts.PathPrefix, "/ws"), wsadapter.Handler(hub))
	}

	// Users API
	mux.HandleFunc(withPrefix(opts.PathPrefix, "/users/"), func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			writeError(w, http.StatusNotFound, "not_found", "route not found", nil)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, opts.PathPrefix)
		if path == "" || path[0] != '/' {
			path = "/" + path
		}
		parts := split(path, '/')
		if len(parts) < 2 {
			writeError(w, http.StatusNotFound, "not_found", "route not found", nil)
			return
		}
		user, err := core.NormalizeUserID(core.UserID(parts[1]))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_user", err.Error(), nil)
			return
		}
		switch r.Method {
		case http.MethodPost:
			if len(parts) >= 3 && parts[2] == "points" {
				metric := core.Metric(r.URL.Query().Get("metric"))
				if metric == "" {
					metric = core.MetricXP
				}
				delta, err := strconv.ParseInt(r.URL.Query().Get("delta"), 10, 64)
				if err != nil {
					writeError(w, http.StatusBadRequest, "invalid_delta", "delta must be an integer", nil)
					return
				}
				total, err := svc.AddPoints(r.Context(), user, metric, delta)
				if err != nil {
					writeError(w, http.StatusBadRequest, "invalid_input", err.Error(), nil)
					return
				}
				writeJSON(w, map[string]any{"total": total})
				return
			}
			if len(parts) >= 4 && parts[2] == "badges" {
				badge := core.Badge(parts[3])
				if err := core.ValidateBadgeID(badge); err != nil {
					writeError(w, http.StatusBadRequest, "invalid_badge", err.Error(), nil)
					return
				}
				if err := svc.AwardBadge(r.Context(), user, badge); err != nil {
					writeError(w, http.StatusBadRequest, "invalid_input", err.Error(), nil)
					return
				}
				writeJSON(w, map[string]any{"ok": true})
				return
			}
		case http.MethodGet:
			st, err := svc.GetState(r.Context(), user)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal", err.Error(), nil)
				return
			}
			writeJSON(w, st)
			return
		}
		writeError(w, http.StatusNotFound, "not_found", "route not found", nil)
	})

	var handler http.Handler = mux
	if opts.AllowCORSOrigin != "" {
		handler = withCORS(handler, opts.AllowCORSOrigin)
	}
	if len(opts.APIKeys) > 0 {
		handler = withAPIKeyAuth(handler, opts.APIKeys)
	}
	if opts.RateLimitEnabled && opts.RateLimitRPM > 0 && opts.RateLimitBurst > 0 {
		handler = withRateLimit(handler, opts.RateLimitRPM, opts.RateLimitBurst)
	}
	return handler
}

// Helpers

// healthCheck verifies the service is working properly
func healthCheck(w http.ResponseWriter, r *http.Request, svc *engine.GamifyService) {
	ctx := r.Context()

	// Verify storage works by trying to fetch a dummy user
	// This is a safe, lightweight check that doesn't affect real data
	dummyUser := core.UserID("healthcheck_probe")
	_, err := svc.GetState(ctx, dummyUser)

	status := map[string]any{
		"status": "healthy",
		"checks": map[string]any{
			"storage": "ok",
		},
	}

	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		status["status"] = "unhealthy"
		status["checks"].(map[string]any)["storage"] = "failed"
	} else {
		w.WriteHeader(http.StatusOK)
	}

	writeJSON(w, status)
}

func withPrefix(prefix, path string) string {
	if prefix == "" || prefix == "/" {
		return path
	}
	if prefix[len(prefix)-1] == '/' {
		return prefix[:len(prefix)-1] + path
	}
	return prefix + path
}

func split(p string, sep rune) []string {
	var parts []string
	cur := make([]rune, 0, len(p))
	// trim leading '/'
	for len(p) > 0 && p[0] == '/' {
		p = p[1:]
	}
	for _, r := range p {
		if r == sep {
			if len(cur) > 0 {
				parts = append(parts, string(cur))
				cur = cur[:0]
			}
			continue
		}
		cur = append(cur, r)
	}
	if len(cur) > 0 {
		parts = append(parts, string(cur))
	}
	return parts
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func writeError(w http.ResponseWriter, status int, code, msg string, details any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(apiError{Code: code, Message: msg, Details: details})
}

// withCORS wraps a handler with a minimal CORS policy.
func withCORS(next http.Handler, origin string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// withAPIKeyAuth enforces a shared API key list.
func withAPIKeyAuth(next http.Handler, apiKeys []string) http.Handler {
	allowed := make(map[string]struct{}, len(apiKeys))
	for _, k := range apiKeys {
		k = strings.TrimSpace(k)
		if k != "" {
			allowed[k] = struct{}{}
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := extractAPIKey(r)
		if key == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing API key", nil)
			return
		}
		if _, ok := allowed[key]; !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "invalid API key", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// withRateLimit applies a simple token-bucket limiter per client key.
func withRateLimit(next http.Handler, rpm int, burst int) http.Handler {
	limiter := newRateLimiter(rpm, burst)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := clientKey(r)
		if !limiter.allow(key) {
			writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func extractAPIKey(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	if key := r.Header.Get("X-API-Key"); key != "" {
		return key
	}
	return ""
}

// clientKey uses API key if present, otherwise remote IP.
func clientKey(r *http.Request) string {
	if key := extractAPIKey(r); key != "" {
		return key
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

type rateLimiter struct {
	rpm   float64
	burst float64
	mu    sync.Mutex
	b     map[string]*bucket
}

type bucket struct {
	tokens float64
	last   time.Time
}

func newRateLimiter(rpm, burst int) *rateLimiter {
	return &rateLimiter{
		rpm:   float64(rpm),
		burst: float64(burst),
		b:     make(map[string]*bucket),
	}
}

func (l *rateLimiter) allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.b[key]
	if !ok {
		l.b[key] = &bucket{tokens: l.burst - 1, last: now}
		return true
	}

	elapsed := now.Sub(b.last).Minutes()
	b.tokens += elapsed * l.rpm
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	if b.tokens < 1 {
		b.last = now
		return false
	}
	b.tokens--
	b.last = now
	return true
}
