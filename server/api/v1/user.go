package v1

import (
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/golang-jwt/jwt/v5"
	woomMiddleware "woom/server/api/middleware"
)

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	streamId, err := h.helperCreateStreamId()
	if err != nil {
		woomMiddleware.WriteError(w, r, http.StatusInternalServerError, "stream_id_generation_failed", "无法创建用户流")
		return
	}

	type Claims struct {
		Id string `json:"id"`
		jwt.RegisteredClaims
	}

	timeNow := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, &Claims{
		streamId,
		jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(timeNow.Add(10 * 365 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(timeNow),
			NotBefore: jwt.NewNumericDate(timeNow),
		},
	})
	tokenString, err := token.SignedString([]byte(h.key))
	if err != nil {
		woomMiddleware.WriteError(w, r, http.StatusInternalServerError, "token_generation_failed", "无法创建用户身份")
		return
	}

	render.JSON(w, r, map[string]string{
		"streamId": streamId,
		"token":    tokenString,
	})
}
