package service

import (
	"encoding/json"
	filemodel "github.com/wt-media/wt-media-cloud/internal/modules/files/model"
	"testing"
)

func TestArtworkFileKeepsIdentifierPrecision(t *testing.T) {
	const id = int64(9007199254740993)
	data, err := json.Marshal(ArtworkFile{FileView: filemodel.FileView{ID: id, Name: "实际输出"}, ID: id})
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err = json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result["id"] != "9007199254740993" || result["name"] != "实际输出" {
		t.Fatalf("wire file: %s", data)
	}
}
