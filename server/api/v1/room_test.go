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
	"github.com/redis/go-redis/v9"
)

func TestCreateRoomWritesVersionOneJSON(t *testing.T) {
	redisServer, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start Redis test server: %v", err)
	}
	defer redisServer.Close()

	rdb := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	defer rdb.Close()

	handler := NewHandler(rdb, "secret")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/room/", nil)
	handler.CreateRoom(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var room model.Room
	if err := json.Unmarshal(recorder.Body.Bytes(), &room); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if room.RoomId == "" {
		t.Fatal("expected room id")
	}

	schema, err := rdb.HGet(context.Background(), room.RoomId, helper.RoomSchemaField).Result()
	if err != nil {
		t.Fatalf("read room schema: %v", err)
	}
	if schema != string(helper.RoomSchemaValue) {
		t.Fatalf("unexpected room schema: %s", schema)
	}
	adminJSON, err := rdb.HGet(context.Background(), room.RoomId, model.AdminUniqueKey).Bytes()
	if err != nil {
		t.Fatalf("read room admin: %v", err)
	}
	var admin model.RoomAdmin
	if err := json.Unmarshal(adminJSON, &admin); err != nil {
		t.Fatalf("expected room admin JSON, got %x: %v", adminJSON, err)
	}
	if admin.Owner != room.Owner {
		t.Fatalf("expected owner %q, got %q", room.Owner, admin.Owner)
	}
}
