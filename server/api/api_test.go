package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/redis/go-redis/v9"
)

func newTestAPI() http.Handler {
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	return NewApi(rdb, "test-secret", "http://127.0.0.1:7777", "")
}

func TestHealthzReturnsOKAndRequestID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recorder := httptest.NewRecorder()

	newTestAPI().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if recorder.Header().Get("X-Request-ID") == "" {
		t.Fatal("expected response to contain X-Request-ID")
	}

	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected JSON response, got %q: %v", recorder.Body.String(), err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status=ok, got %q", body["status"])
	}
}

func TestReadyzReturnsServiceUnavailableWhenRedisIsUnavailable(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	recorder := httptest.NewRecorder()

	newTestAPI().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", recorder.Code)
	}
	if recorder.Header().Get("X-Request-ID") == "" {
		t.Fatal("expected response to contain X-Request-ID")
	}
}

func TestUnauthorizedRoomReturnsStructuredError(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/room/", nil)
	recorder := httptest.NewRecorder()

	newTestAPI().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", recorder.Code)
	}

	var body struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"requestId"`
		} `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected JSON response, got %q: %v", recorder.Body.String(), err)
	}
	if body.Error.Code != "unauthorized" || body.Error.Message == "" || body.Error.RequestID == "" {
		t.Fatalf("unexpected error body: %+v", body.Error)
	}
}
