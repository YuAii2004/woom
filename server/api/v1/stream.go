package v1

import (
	"net/http"
	"woom/server/model"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	woomMiddleware "woom/server/api/middleware"
)

func (h *Handler) CreateRoomStream(w http.ResponseWriter, r *http.Request) {
	room, err := h.helperShowRoom(r)
	if err != nil {
		woomMiddleware.WriteError(w, r, http.StatusInternalServerError, "room_read_failed", "无法读取会议")
		return
	}

	streamId, err := h.helperCreateStreamId()
	if err != nil {
		woomMiddleware.WriteError(w, r, http.StatusInternalServerError, "stream_id_generation_failed", "无法创建会议流")
		return
	}
	if err := h.helperSetRoomStream(r, room.RoomId, streamId, &model.Stream{}); err != nil {
		woomMiddleware.WriteError(w, r, http.StatusInternalServerError, "stream_persistence_failed", "无法加入会议")
		return
	}
	room.StreamId = streamId
	render.JSON(w, r, room)
}

func (h *Handler) UpdateRoomStream(w http.ResponseWriter, r *http.Request) {
	streamId := chi.URLParam(r, "streamId")
	room, err := h.helperShowRoom(r)
	if err != nil {
		woomMiddleware.WriteError(w, r, http.StatusInternalServerError, "room_read_failed", "无法读取会议")
		return
	}

	stream := &model.Stream{}
	if err := render.DecodeJSON(r.Body, stream); err != nil {
		woomMiddleware.WriteError(w, r, http.StatusBadRequest, "invalid_request", "请求格式不正确")
		return
	}

	if err := h.helperSetRoomStream(r, room.RoomId, streamId, stream); err != nil {
		woomMiddleware.WriteError(w, r, http.StatusInternalServerError, "stream_persistence_failed", "无法更新会议流")
		return
	}
	room.StreamId = streamId
	render.JSON(w, r, room)
}

func (h *Handler) DestroyRoomStream(w http.ResponseWriter, r *http.Request) {
	roomId := chi.URLParam(r, "roomId")
	streamId := chi.URLParam(r, "streamId")

	if err := h.rdb.HDel(r.Context(), roomId, streamId).Err(); err != nil {
		woomMiddleware.WriteError(w, r, http.StatusInternalServerError, "stream_deletion_failed", "无法离开会议")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
