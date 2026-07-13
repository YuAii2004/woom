package helper

import (
	"encoding/json"
	"fmt"
)

const (
	RoomSchemaField = "__schema"
	RoomValueJSON   = "json"
	RoomValueGob    = "gob"
)

var RoomSchemaValue = []byte(`{"version":1,"encoding":"json"}`)

type RoomSchema struct {
	Version  int    `json:"version"`
	Encoding string `json:"encoding"`
}

func EncodeRoomValue(value any) ([]byte, error) {
	return json.Marshal(value)
}

func DecodeRoomValue(data []byte, target any) (string, error) {
	if err := json.Unmarshal(data, target); err == nil {
		return RoomValueJSON, nil
	}

	if err := GobDecode(target, data); err != nil {
		return "", fmt.Errorf("room value is neither JSON nor Gob: %w", err)
	}
	return RoomValueGob, nil
}

func DecodeRoomJSONValue(data []byte, target any) error {
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("invalid room JSON value: %w", err)
	}
	return nil
}

func ReadRoomSchema(value string) (bool, RoomSchema, error) {
	if value == "" {
		return false, RoomSchema{}, nil
	}

	var schema RoomSchema
	if err := json.Unmarshal([]byte(value), &schema); err != nil {
		return true, RoomSchema{}, fmt.Errorf("invalid room schema: %w", err)
	}
	if schema.Version != 1 || schema.Encoding != RoomValueJSON {
		return true, schema, fmt.Errorf("unsupported room schema: version=%d encoding=%q", schema.Version, schema.Encoding)
	}
	return true, schema, nil
}
