package observability

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	woomMiddleware "woom/server/api/middleware"
)

func TestRequestLoggerWritesCorrelatedStructuredEvent(t *testing.T) {
	var output bytes.Buffer
	logger := NewLogger(&output)
	handler := woomMiddleware.RequestID(RequestLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))

	req := httptest.NewRequest(http.MethodGet, "/room/123-456-789", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", recorder.Code)
	}
	if output.Len() == 0 {
		t.Fatal("expected a structured log event")
	}

	var event map[string]any
	if err := json.Unmarshal(output.Bytes(), &event); err != nil {
		t.Fatalf("expected one JSON log event, got %q: %v", output.String(), err)
	}
	for _, field := range []string{"timestamp", "service", "event", "operation", "request_id", "status", "duration_ms"} {
		if _, ok := event[field]; !ok {
			t.Fatalf("missing log field %q in %s", field, output.String())
		}
	}
	if event["request_id"] != recorder.Header().Get("X-Request-ID") {
		t.Fatalf("request id was not correlated: event=%v header=%q", event["request_id"], recorder.Header().Get("X-Request-ID"))
	}
	if strings.Contains(output.String(), "secret-token") || strings.Contains(output.String(), "Authorization") {
		t.Fatalf("sensitive authorization data leaked into log: %s", output.String())
	}
}
