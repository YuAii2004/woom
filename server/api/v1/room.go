package v1

import (
	"net/http"

	woomMiddleware "woom/server/api/middleware"
	"woom/server/helper"
	"woom/server/model"

	"github.com/go-chi/render"
)

const idLength = 9

func (h *Handler) CreateRoom(w http.ResponseWriter, r *http.Request) {
	roomId := helper.AddSplitSymbol(helper.GenNumberSecret(idLength))
	streamId, err := h.helperCreateStreamId()
	if err != nil {
		woomMiddleware.WriteError(w, r, http.StatusInternalServerError, "stream_id_generation_failed", "无法创建会议流")
		return
	}

	admin := model.RoomAdmin{
		Owner:     streamId,
		Presenter: "",
		Locked:    false,
	}
	jsonAdmin, err := helper.EncodeRoomValue(&admin)
	if err != nil {
		woomMiddleware.WriteError(w, r, http.StatusInternalServerError, "room_encoding_failed", "无法保存会议状态")
		return
	}
	if err := h.rdb.HSet(r.Context(), roomId, model.AdminUniqueKey, jsonAdmin, helper.RoomSchemaField, helper.RoomSchemaValue).Err(); err != nil {
		woomMiddleware.WriteError(w, r, http.StatusInternalServerError, "room_persistence_failed", "无法创建会议")
		return
	}

	//if _, err := h.helperCreateRoomStream(r, roomId, streamId); err != nil {
	//	w.WriteHeader(http.StatusInternalServerError)
	//	w.Write([]byte(err.Error()))
	//	return
	//}

	room := model.Room{
		RoomId:    roomId,
		RoomAdmin: admin,
		StreamId:  streamId,
	}
	render.JSON(w, r, room)
}

func (h *Handler) ShowRoom(w http.ResponseWriter, r *http.Request) {
	room, err := h.helperShowRoom(r)
	if err != nil {
		woomMiddleware.WriteError(w, r, http.StatusInternalServerError, "room_read_failed", "无法读取会议")
		return
	}
	render.JSON(w, r, room)
}
