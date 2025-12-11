package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	mem "gamifykit/adapters/memory"
	"gamifykit/engine"
)

func TestAddPointsSuccess(t *testing.T) {
	svc := newTestService()
	handler := NewMux(svc, nil, Options{PathPrefix: "/api"})

	req := httptest.NewRequest(http.MethodPost, "/api/users/alice/points?metric=xp&delta=10", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["total"] != float64(10) {
		t.Fatalf("expected total 10, got %v", resp["total"])
	}
}

func TestAddPointsValidation(t *testing.T) {
	svc := newTestService()
	handler := NewMux(svc, nil, Options{PathPrefix: "/api"})

	req := httptest.NewRequest(http.MethodPost, "/api/users/alice/points?delta=bad", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestAwardBadgeValidation(t *testing.T) {
	svc := newTestService()
	handler := NewMux(svc, nil, Options{PathPrefix: "/api"})

	req := httptest.NewRequest(http.MethodPost, "/api/users/alice/badges/%20", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGetUserNotFound(t *testing.T) {
	svc := newTestService()
	handler := NewMux(svc, nil, Options{PathPrefix: "/api"})

	req := httptest.NewRequest(http.MethodGet, "/api/users/unknown", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAPIKeyAuth(t *testing.T) {
	svc := newTestService()
	handler := NewMux(svc, nil, Options{
		PathPrefix:      "/api",
		APIKeys:         []string{"secret"},
		AllowCORSOrigin: "*",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users/alice", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/users/alice", nil)
	req2.Header.Set("Authorization", "Bearer secret")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec2.Code)
	}
}

func TestRateLimit(t *testing.T) {
	svc := newTestService()
	handler := NewMux(svc, nil, Options{
		PathPrefix:       "/api",
		APIKeys:          []string{"k"},
		RateLimitEnabled: true,
		RateLimitRPM:     1,
		RateLimitBurst:   1,
	})

	req1 := httptest.NewRequest(http.MethodGet, "/api/users/alice", nil)
	req1.Header.Set("X-API-Key", "k")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected 200 first request, got %d", rec1.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/users/alice", nil)
	req2.Header.Set("X-API-Key", "k")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec2.Code)
	}
}

func newTestService() *engine.GamifyService {
	storage := mem.New()
	bus := engine.NewEventBus(engine.DispatchSync)
	rules := engine.DefaultRuleEngine()
	return engine.NewGamifyService(storage, bus, rules)
}
