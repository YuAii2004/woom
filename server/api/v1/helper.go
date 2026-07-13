package v1

import (
	"fmt"
	"net/http"

	"woom/server/helper"
	"woom/server/model"

	"github.com/go-chi/chi/v5"
	"github.com/gofrs/uuid/v5"
)

func (h *Handler) helperCreateStreamId() (string, error) {
	id, err := uuid.NewV4()
	return id.String(), err
}

func (h *Handler) helperSetRoomStream(r *http.Request, roomId, streamId string, stream *model.Stream) error {
	jsonStream, err := helper.EncodeRoomValue(stream)
	if err != nil {
		return err
	}

	return h.rdb.HSet(r.Context(), roomId, streamId, jsonStream, helper.RoomSchemaField, helper.RoomSchemaValue).Err()
}

func (h *Handler) helperShowRoom(r *http.Request) (*model.Room, error) {
	roomId := chi.URLParam(r, "roomId")
	room := model.Room{
		RoomId:  roomId,
		Streams: map[string]model.Stream{},
	}
	result, err := h.rdb.HGetAll(r.Context(), roomId).Result()
	if err != nil {
		return &room, err
	}

	schemaPresent, _, err := helper.ReadRoomSchema(result[helper.RoomSchemaField])
	if err != nil {
		return &room, err
	}
	canonicalValues := make(map[string][]byte)
	for k, v := range result {
		if k == helper.RoomSchemaField {
			continue
		}

		var target any
		var stream model.Stream
		if k == model.AdminUniqueKey {
			target = &room.RoomAdmin
		} else {
			target = &stream
		}

		if schemaPresent {
			if err := helper.DecodeRoomJSONValue([]byte(v), target); err != nil {
				return &room, fmt.Errorf("decode room field %q: %w", k, err)
			}
		} else {
			if _, err := helper.DecodeRoomValue([]byte(v), target); err != nil {
				return &room, fmt.Errorf("decode legacy room field %q: %w", k, err)
			}
			canonical, err := helper.EncodeRoomValue(target)
			if err != nil {
				return &room, fmt.Errorf("encode migrated room field %q: %w", k, err)
			}
			canonicalValues[k] = canonical
		}
		if k != model.AdminUniqueKey {
			room.Streams[k] = stream
		}
	}

	if !schemaPresent && len(canonicalValues) > 0 {
		values := []any{helper.RoomSchemaField, helper.RoomSchemaValue}
		for field, value := range canonicalValues {
			values = append(values, field, value)
		}
		if err := h.rdb.HSet(r.Context(), roomId, values...).Err(); err != nil {
			return &room, fmt.Errorf("migrate room %q: %w", roomId, err)
		}
	}

	return &room, nil
}
