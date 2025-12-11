package webhook

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"gamifykit/core"
)

func TestSink_OnEventPostsToEndpoints(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = io.ReadAll(r.Body)
		_ = r.Body.Close()
	}))
	defer srv.Close()

	sink := New([]string{srv.URL})
	sink.OnEvent(core.NewPointsAdded("u1", core.MetricXP, 5, 5))

	if atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("expected 1 hit, got %d", hits)
	}
}
