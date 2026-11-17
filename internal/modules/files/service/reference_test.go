package service

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestReferenceTokensRecognizesSavedNestedURLs(t *testing.T) {
	got, err := referenceTokens(json.RawMessage(`{"intro":"http://127.0.0.1:8188/api/v1/file-resources/b?x=1","nested":[{"cover":"/api/v1/file-resources/a"},"/api/v1/file-resources/a"],"external":"https://example.com/video.mp4"}`))
	if err != nil || !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("%v %v", got, err)
	}
	if _, err = referenceTokens(json.RawMessage(`{`)); err == nil {
		t.Fatal("invalid snapshot accepted")
	}
}
