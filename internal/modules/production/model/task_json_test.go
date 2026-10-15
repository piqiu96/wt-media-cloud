package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestComposeTaskAPIProjectionDoesNotExposeRawExecutionErrors(t *testing.T) {
	for _, value := range []any{
		ComposeTask{ErrorCode: "storage_error", ErrorMessage: "signed=https://storage.example/object?token=secret"},
		ComposeTaskAttempt{ErrorCode: "execution_uncertain", ErrorMessage: "ffmpeg -i /private/source.mp4 token=secret"},
	} {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		body := string(encoded)
		if strings.Contains(body, "storage.example") || strings.Contains(body, "token=secret") || strings.Contains(body, "/private/source.mp4") {
			t.Fatalf("raw execution detail leaked in API body: %s", body)
		}
		if !strings.Contains(body, `"error_message"`) {
			t.Fatalf("contract error_message field vanished: %s", body)
		}
	}
}
