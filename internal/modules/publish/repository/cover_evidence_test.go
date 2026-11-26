package repository

import (
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"testing"
)

func TestProgressRequiresTheExactPlatformSecondaryCover(t *testing.T) {
	a := model.Attempt{Input: model.ExecutionInput{PlatformUID: "u", BitProfileID: "p", VideoSHA256: "v", CoverSHA256: "c", SpaceCoverSHA256: "space"}}
	r := model.ExecutionReport{PlatformUID: "u", BitProfileID: "p", VideoSHA256: "v", CoverSHA256: "c"}
	if verified(a, r) {
		t.Fatal("missing Bilibili space cover was accepted")
	}
	r.SpaceCoverSHA256 = "wrong"
	if verified(a, r) {
		t.Fatal("changed Bilibili space cover was accepted")
	}
	r.SpaceCoverSHA256 = "space"
	if !verified(a, r) {
		t.Fatal("matching space cover was rejected")
	}
	r.PortraitCoverSHA256 = "foreign"
	if verified(a, r) {
		t.Fatal("foreign platform evidence was accepted")
	}
}
