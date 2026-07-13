package api_test

import (
	"encoding/json"
	"testing"

	"woom/server/model"
)

func TestContractModelsKeepStableJSONFields(t *testing.T) {
	room := model.Room{
		RoomId: "123-456-789",
		RoomAdmin: model.RoomAdmin{
			Owner:     "owner-id",
			Presenter: "presenter-id",
			Locked:    true,
		},
		StreamId: "stream-id",
		Streams: map[string]model.Stream{
			"stream-id": {
				Name:   "Alice",
				State:  "connected",
				Audio:  true,
				Video:  true,
				Screen: false,
			},
		},
	}
	user := model.User{StreamId: "stream-id", Token: "jwt"}
	stream := model.Stream{
		Name:   "Alice",
		State:  "connected",
		Audio:  true,
		Video:  true,
		Screen: false,
	}
	errorResponse := model.ErrorResponse{
		Error: model.ErrorDetail{
			Code:      "invalid_request",
			Message:   "请求格式不正确",
			RequestID: "request-id",
		},
	}

	tests := []struct {
		name     string
		value    any
		expected []string
	}{
		{name: "room", value: room, expected: []string{"roomId", "owner", "presenter", "locked", "streamId", "streams"}},
		{name: "user", value: user, expected: []string{"streamId", "token"}},
		{name: "stream", value: stream, expected: []string{"name", "state", "audio", "video", "screen"}},
		{name: "error", value: errorResponse, expected: []string{"error"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, err := json.Marshal(test.value)
			if err != nil {
				t.Fatalf("marshal failed: %v", err)
			}

			var fields map[string]json.RawMessage
			if err := json.Unmarshal(data, &fields); err != nil {
				t.Fatalf("decode failed: %v", err)
			}
			for _, field := range test.expected {
				if _, ok := fields[field]; !ok {
					t.Fatalf("missing JSON field %q in %s", field, data)
				}
			}
		})
	}
}
