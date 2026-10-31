package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/lidisoft/horizon/internal/platform/metrics"
)

func engine(checks ...Check) http.Handler {
	return NewEngine(Deps{Log: zerolog.Nop(), Checks: checks, Metrics: metrics.New()})
}

func get(t *testing.T, h http.Handler, path string, header http.Header) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range header {
		req.Header[k] = v
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHealthz(t *testing.T) {
	rec := get(t, engine(), "/healthz", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}

func TestReadyzReportsEachCheck(t *testing.T) {
	ok := Check{Name: "postgres", Check: func(context.Context) error { return nil }}
	bad := Check{Name: "s3", Check: func(context.Context) error { return errors.New(`bucket "horizon" does not exist`) }}

	rec := get(t, engine(ok), "/readyz", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("all ok: got %d %s", rec.Code, rec.Body)
	}

	rec = get(t, engine(ok, bad), "/readyz", nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("one failing: got %d", rec.Code)
	}
	var body ReadyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "not ready" || body.Checks["postgres"] != "ok" || body.Checks["s3"] != `bucket "horizon" does not exist` {
		t.Errorf("body = %+v", body)
	}
}

func TestRequestID(t *testing.T) {
	h := engine()

	rec := get(t, h, "/healthz", nil)
	if id := rec.Header().Get(HeaderRequestID); len(id) != 36 {
		t.Errorf("generated id = %q, want a UUID", id)
	}

	rec = get(t, h, "/healthz", http.Header{HeaderRequestID: {"edge-1234"}})
	if id := rec.Header().Get(HeaderRequestID); id != "edge-1234" {
		t.Errorf("forwarded id = %q", id)
	}

	rec = get(t, h, "/healthz", http.Header{HeaderRequestID: {"bad id\n"}})
	if id := rec.Header().Get(HeaderRequestID); id == "bad id\n" || len(id) != 36 {
		t.Errorf("malformed id kept: %q", id)
	}
}

func TestMetricsCountRequestsByRoute(t *testing.T) {
	reg := metrics.New()
	h := NewEngine(Deps{Log: zerolog.Nop(), Metrics: reg})
	get(t, h, "/healthz", nil)
	get(t, h, "/nope", nil)

	rec := get(t, reg.Handler(), "/metrics", nil)
	body := rec.Body.String()
	for _, want := range []string{
		`horizon_http_requests_total{method="GET",route="/healthz",status="200"} 1`,
		`horizon_http_requests_total{method="GET",route="unmatched",status="404"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics missing %s", want)
		}
	}
}
