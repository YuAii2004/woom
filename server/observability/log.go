package observability

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	woomMiddleware "woom/server/api/middleware"
)

type ClientEvent struct {
	Timestamp string         `json:"timestamp,omitempty"`
	Level     string         `json:"level"`
	Service   string         `json:"service"`
	Event     string         `json:"event"`
	Component string         `json:"component,omitempty"`
	Operation string         `json:"operation,omitempty"`
	RequestID string         `json:"request_id,omitempty"`
	SessionID string         `json:"session_id,omitempty"`
	RoomID    string         `json:"room_id,omitempty"`
	StreamID  string         `json:"stream_id,omitempty"`
	Browser   string         `json:"browser,omitempty"`
	OS        string         `json:"os,omitempty"`
	Duration  int64          `json:"duration_ms,omitempty"`
	Error     map[string]any `json:"error,omitempty"`
	Context   map[string]any `json:"context,omitempty"`
}

var safeContextFields = map[string]bool{
	"browser":          true,
	"connection_state": true,
	"device_kind":      true,
	"feature":          true,
	"http_status":      true,
	"os":               true,
	"permission_state": true,
	"retry_count":      true,
}

var safeErrorFields = map[string]bool{
	"code":    true,
	"kind":    true,
	"message": true,
	"name":    true,
	"status":  true,
}

func NewLogger(output io.Writer) *slog.Logger {
	if output == nil {
		output = io.Discard
	}
	handler := slog.NewJSONHandler(output, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			if len(groups) > 0 {
				return attr
			}
			switch attr.Key {
			case slog.TimeKey:
				attr.Key = "logged_at"
			case slog.LevelKey:
				attr.Value = slog.StringValue(strings.ToLower(attr.Value.String()))
			}
			return attr
		},
	})
	return slog.New(handler)
}

func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = NewLogger(nil)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			writer := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(writer, r)

			logger.LogAttrs(r.Context(), slog.LevelInfo, "http_request",
				slog.String("timestamp", time.Now().UTC().Format(time.RFC3339Nano)),
				slog.String("service", "woom-go"),
				slog.String("event", "http_request"),
				slog.String("component", "http"),
				slog.String("operation", r.Method+" "+r.URL.Path),
				slog.String("request_id", woomMiddleware.RequestIDFromRequest(r)),
				slog.Int("status", writer.status),
				slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			)
		})
	}
}

func LogClientEvent(logger *slog.Logger, ctx context.Context, event ClientEvent) {
	if logger == nil {
		logger = NewLogger(nil)
	}
	timestamp := event.Timestamp
	if timestamp == "" {
		timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	level := slog.LevelInfo
	switch strings.ToLower(event.Level) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	attrs := []slog.Attr{
		slog.String("timestamp", timestamp),
		slog.String("service", event.Service),
		slog.String("event", event.Event),
		slog.String("component", event.Component),
		slog.String("operation", event.Operation),
		slog.String("request_id", event.RequestID),
		slog.String("session_id", event.SessionID),
		slog.String("room_id", event.RoomID),
		slog.String("stream_id", event.StreamID),
		slog.String("browser", event.Browser),
		slog.String("os", event.OS),
		slog.Int64("duration_ms", event.Duration),
		slog.Any("context", sanitizeFields(event.Context, safeContextFields)),
		slog.Any("error", sanitizeFields(event.Error, safeErrorFields)),
	}
	logger.LogAttrs(ctx, level, "client_event", attrs...)
}

func sanitizeFields(fields map[string]any, allowed map[string]bool) map[string]any {
	if len(fields) == 0 {
		return nil
	}
	result := make(map[string]any)
	for key, value := range fields {
		if !allowed[key] {
			continue
		}
		switch typed := value.(type) {
		case string, bool:
			result[key] = typed
		case float64:
			result[key] = typed
		case json.Number:
			result[key] = typed.String()
		case int:
			result[key] = strconv.Itoa(typed)
		}
	}
	return result
}

type statusWriter struct {
	http.ResponseWriter
	status  int
	written bool
}

func (w *statusWriter) WriteHeader(status int) {
	if w.written {
		return
	}
	w.status = status
	w.written = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(body []byte) (int, error) {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func (w *statusWriter) Flush() {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
