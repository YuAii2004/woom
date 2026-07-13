package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	woomMiddleware "woom/server/api/middleware"
	"woom/server/observability"
)

const maxClientEventBytes = 16 * 1024

func clientEventsHandler(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxClientEventBytes)
		var event observability.ClientEvent
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&event); err != nil {
			var maxBytesError *http.MaxBytesError
			if errors.As(err, &maxBytesError) {
				woomMiddleware.WriteError(w, r, http.StatusRequestEntityTooLarge, "payload_too_large", "诊断事件超过大小限制")
				return
			}
			woomMiddleware.WriteError(w, r, http.StatusBadRequest, "invalid_request", "诊断事件格式不正确")
			return
		}
		if err := validateClientEvent(event); err != nil {
			woomMiddleware.WriteError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		if err := ensureSingleJSONValue(decoder); err != nil {
			woomMiddleware.WriteError(w, r, http.StatusBadRequest, "invalid_request", "诊断事件只能包含一个 JSON 对象")
			return
		}

		observability.LogClientEvent(logger, r.Context(), event)
		w.WriteHeader(http.StatusAccepted)
	}
}

func validateClientEvent(event observability.ClientEvent) error {
	if event.Level != "debug" && event.Level != "info" && event.Level != "warn" && event.Level != "error" {
		return errors.New("诊断事件级别不正确")
	}
	if event.Service == "" || event.Event == "" {
		return errors.New("诊断事件缺少 service 或 event")
	}
	return nil
}

func ensureSingleJSONValue(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("诊断事件包含多余内容")
	}
	return nil
}
