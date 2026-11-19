package model

import (
	"encoding/json"
	"os"
	"testing"
)

func TestCompletionHashMatchesPersistedPythonCanonicalPackets(t *testing.T) {
	raw, e := os.ReadFile("testdata/completion-hash.json")
	if e != nil {
		t.Fatal(e)
	}
	var vectors []struct {
		Seq      uint64          `json:"seq"`
		Result   json.RawMessage `json:"result"`
		Expected string          `json:"expected"`
	}
	if e = json.Unmarshal(raw, &vectors); e != nil {
		t.Fatal(e)
	}
	for _, v := range vectors {
		got, e := CompletionHash(v.Seq, v.Result)
		if e != nil || got != v.Expected {
			t.Fatalf("got %s expected %s err %v", got, v.Expected, e)
		}
	}
	for _, raw := range []string{`{} {}`, `{"x":1.5}`, `{"x":1e2}`, `{"x":NaN}`} {
		if _, e := CompletionHash(1, json.RawMessage(raw)); e == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
func TestResultDoesNotAdmitCredentialsOrUnknownFacts(t *testing.T) {
	for _, raw := range []string{`{"status":"uncertain","cookie":"secret"}`, `{"status":"uncertain","evidence":{"download_url":"secret"}}`, `{"status":"uncertain"} {}`} {
		if _, e := ParseResult(json.RawMessage(raw)); e == nil {
			t.Fatal("unknown evidence accepted")
		}
	}
}
