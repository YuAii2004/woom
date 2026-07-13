package v1

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"woom/server/helper"
	"woom/server/model"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

func roomRequest(roomID string) *http.Request {
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("roomId", roomID)
	request := httptest.NewRequest(http.MethodGet, "/room/"+roomID, nil)
	return request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))
}

func TestHelperShowRoomMigratesLegacyGobToJSON(t *testing.T) {
	redisServer, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start Redis test server: %v", err)
	}
	defer redisServer.Close()

	rdb := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	defer rdb.Close()

	roomID := "123-456-789"
	streamID := "stream-id"
	admin := model.RoomAdmin{Owner: "owner-id", Locked: true}
	stream := model.Stream{Name: "Alice", State: "connected", Audio: true, Video: true}
	adminGob, err := helper.GobEncode(&admin)
	if err != nil {
		t.Fatalf("encode admin Gob: %v", err)
	}
	streamGob, err := helper.GobEncode(&stream)
	if err != nil {
		t.Fatalf("encode stream Gob: %v", err)
	}
	if err := rdb.HSet(context.Background(), roomID, model.AdminUniqueKey, adminGob, streamID, streamGob).Err(); err != nil {
		t.Fatalf("seed legacy room: %v", err)
	}

	handler := NewHandler(rdb, "secret")
	room, err := handler.helperShowRoom(roomRequest(roomID))
	if err != nil {
		t.Fatalf("show legacy room: %v", err)
	}
	if room.RoomAdmin != admin || room.Streams[streamID] != stream {
		t.Fatalf("unexpected room: %+v", room)
	}

	schema, err := rdb.HGet(context.Background(), roomID, helper.RoomSchemaField).Result()
	if err != nil {
		t.Fatalf("read migrated schema: %v", err)
	}
	if schema != string(helper.RoomSchemaValue) {
		t.Fatalf("unexpected schema: %s", schema)
	}

	var migratedAdmin model.RoomAdmin
	adminJSON, err := rdb.HGet(context.Background(), roomID, model.AdminUniqueKey).Bytes()
	if err != nil {
		t.Fatalf("read migrated admin: %v", err)
	}
	if err := json.Unmarshal(adminJSON, &migratedAdmin); err != nil {
		t.Fatalf("expected migrated admin JSON, got %x: %v", adminJSON, err)
	}
	if migratedAdmin != admin {
		t.Fatalf("unexpected migrated admin: %+v", migratedAdmin)
	}
}

func TestHelperShowRoomReadsVersionOneJSON(t *testing.T) {
	redisServer, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start Redis test server: %v", err)
	}
	defer redisServer.Close()

	rdb := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	defer rdb.Close()

	roomID := "123-456-789"
	admin := model.RoomAdmin{Owner: "owner-id"}
	adminJSON, err := helper.EncodeRoomValue(&admin)
	if err != nil {
		t.Fatalf("encode admin JSON: %v", err)
	}
	if err := rdb.HSet(context.Background(), roomID, helper.RoomSchemaField, helper.RoomSchemaValue, model.AdminUniqueKey, adminJSON).Err(); err != nil {
		t.Fatalf("seed JSON room: %v", err)
	}

	handler := NewHandler(rdb, "secret")
	room, err := handler.helperShowRoom(roomRequest(roomID))
	if err != nil {
		t.Fatalf("show JSON room: %v", err)
	}
	if room.Owner != admin.Owner {
		t.Fatalf("expected owner %q, got %q", admin.Owner, room.Owner)
	}
}

func TestHelperShowRoomMigratesMixedRoomIdempotently(t *testing.T) {
	redisServer, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start Redis test server: %v", err)
	}
	defer redisServer.Close()

	rdb := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	defer rdb.Close()

	roomID := "123-456-789"
	streamID := "stream-id"
	admin := model.RoomAdmin{Owner: "owner-id"}
	stream := model.Stream{Name: "Alice", State: "connected", Audio: true}
	adminJSON, err := helper.EncodeRoomValue(&admin)
	if err != nil {
		t.Fatalf("encode admin JSON: %v", err)
	}
	streamGob, err := helper.GobEncode(&stream)
	if err != nil {
		t.Fatalf("encode stream Gob: %v", err)
	}
	if err := rdb.HSet(context.Background(), roomID, model.AdminUniqueKey, adminJSON, streamID, streamGob).Err(); err != nil {
		t.Fatalf("seed mixed room: %v", err)
	}

	handler := NewHandler(rdb, "secret")
	first, err := handler.helperShowRoom(roomRequest(roomID))
	if err != nil {
		t.Fatalf("show mixed room: %v", err)
	}
	second, err := handler.helperShowRoom(roomRequest(roomID))
	if err != nil {
		t.Fatalf("show migrated room a second time: %v", err)
	}
	if first.RoomAdmin != second.RoomAdmin || first.Streams[streamID] != second.Streams[streamID] {
		t.Fatalf("room changed between reads: first=%+v second=%+v", first, second)
	}
	if schema, err := rdb.HGet(context.Background(), roomID, helper.RoomSchemaField).Result(); err != nil || schema != string(helper.RoomSchemaValue) {
		t.Fatalf("unexpected migrated schema: %q, %v", schema, err)
	}
}

func TestHelperShowRoomRejectsMalformedVersionOneValue(t *testing.T) {
	redisServer, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start Redis test server: %v", err)
	}
	defer redisServer.Close()

	rdb := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	defer rdb.Close()

	roomID := "123-456-789"
	if err := rdb.HSet(context.Background(), roomID, helper.RoomSchemaField, helper.RoomSchemaValue, model.AdminUniqueKey, "not-json").Err(); err != nil {
		t.Fatalf("seed malformed room: %v", err)
	}

	handler := NewHandler(rdb, "secret")
	if _, err := handler.helperShowRoom(roomRequest(roomID)); err == nil {
		t.Fatal("expected malformed JSON room value to fail")
	}
}
