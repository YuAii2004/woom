package helper

import (
	"testing"

	"woom/server/model"
)

func TestDecodeRoomValuePrefersJSON(t *testing.T) {
	var stream model.Stream
	encoding, err := DecodeRoomValue([]byte(`{"name":"Alice","state":"connected","audio":true,"video":true,"screen":false}`), &stream)
	if err != nil {
		t.Fatalf("decode JSON failed: %v", err)
	}
	if encoding != RoomValueJSON {
		t.Fatalf("expected JSON encoding, got %q", encoding)
	}
	if stream.Name != "Alice" || !stream.Audio || !stream.Video {
		t.Fatalf("unexpected stream: %+v", stream)
	}
}

func TestDecodeRoomValueFallsBackToGob(t *testing.T) {
	expected := model.RoomAdmin{Owner: "owner-id", Locked: true}
	data, err := GobEncode(&expected)
	if err != nil {
		t.Fatalf("encode Gob failed: %v", err)
	}

	var actual model.RoomAdmin
	encoding, err := DecodeRoomValue(data, &actual)
	if err != nil {
		t.Fatalf("decode Gob failed: %v", err)
	}
	if encoding != RoomValueGob {
		t.Fatalf("expected Gob encoding, got %q", encoding)
	}
	if actual != expected {
		t.Fatalf("expected %+v, got %+v", expected, actual)
	}
}

func TestReadRoomSchemaAcceptsVersionOne(t *testing.T) {
	present, schema, err := ReadRoomSchema(string(RoomSchemaValue))
	if err != nil {
		t.Fatalf("read schema failed: %v", err)
	}
	if !present || schema.Version != 1 || schema.Encoding != "json" {
		t.Fatalf("unexpected schema: present=%v schema=%+v", present, schema)
	}
}

func TestReadRoomSchemaRejectsUnsupportedVersion(t *testing.T) {
	present, _, err := ReadRoomSchema(`{"version":2,"encoding":"json"}`)
	if err == nil {
		t.Fatal("expected unsupported schema to fail")
	}
	if !present {
		t.Fatal("expected malformed schema to be marked present")
	}
}
