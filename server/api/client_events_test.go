package api

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"woom/server/observability"
)

func TestClientEventsAcceptSafeEventAndDropSensitiveContext(t *testing.T) {
	var logs bytes.Buffer
	logger := observability.NewLogger(&logs)
	handler := clientEventsHandler(logger)
	req := httptest.NewRequest(http.MethodPost, "/client-events", strings.NewReader(`{
        "level":"error",
        "service":"woom-web",
        "event":"whip_failed",
        "component":"media",
        "operation":"publish",
        "request_id":"request-id",
        "session_id":"session-id",
        "room_id":"room-id",
        "stream_id":"stream-id",
        "context":{"http_status":503,"token":"secret-token"}
    }`))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(logs.String(), "secret-token") || strings.Contains(logs.String(), "token") {
		t.Fatalf("sensitive event data leaked into log: %s", logs.String())
	}
	if !strings.Contains(logs.String(), "whip_failed") || !strings.Contains(logs.String(), "503") {
		t.Fatalf("expected safe event fields in log: %s", logs.String())
	}
}

func TestClientEventsRejectOversizedPayload(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	handler := clientEventsHandler(logger)
	payload := `{"level":"error","service":"web","event":"` + strings.Repeat("x", 17*1024) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/client-events", strings.NewReader(payload))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status 413, got %d", recorder.Code)
	}
}
