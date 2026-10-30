package identity

import (
	"encoding/json"
	"testing"
)

func TestProxyIDUsesNumericJSON(t *testing.T) {
	type value struct {
		ID ProxyID `json:"proxy_id"`
	}
	encoded, err := json.Marshal(value{ID: "42"})
	if err != nil || string(encoded) != `{"proxy_id":42}` {
		t.Fatalf("encoded=%s err=%v", encoded, err)
	}
	var decoded value
	if err := json.Unmarshal(encoded, &decoded); err != nil || decoded.ID != "42" {
		t.Fatalf("decoded=%+v err=%v", decoded, err)
	}
	if err := json.Unmarshal([]byte(`{"proxy_id":"legacy-text"}`), &decoded); err == nil {
		t.Fatal("non-numeric proxy id was accepted")
	}
}
