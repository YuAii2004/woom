package middleware

import (
	"encoding/json"
	"net/http"
	"woom/server/model"
)

func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(model.ErrorResponse{
		Error: model.ErrorDetail{
			Code:      code,
			Message:   message,
			RequestID: RequestIDFromRequest(r),
		},
	})
}
