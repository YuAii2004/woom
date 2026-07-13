package middleware

import (
	"encoding/json"
	"net/http"
)

type errorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId"`
}

type errorResponse struct {
	Error errorBody `json:"error"`
}

func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse{
		Error: errorBody{
			Code:      code,
			Message:   message,
			RequestID: RequestIDFromRequest(r),
		},
	})
}
