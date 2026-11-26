package model

import (
	"testing"
	"time"
)

func TestBilibiliPublicIdentityIsBoundToTheExactOriginalAID(t *testing.T) {
	const aid = "117590282275872"
	canonical := "https://www.bilibili.com/video/av" + aid
	for _, raw := range []string{canonical, canonical + "/"} {
		if !ExactBilibiliWorkURL(aid, raw) {
			t.Fatal("canonical rejected", raw)
		}
	}
	for _, raw := range []string{
		"http://www.bilibili.com/video/av" + aid, canonical + "?share=1", canonical + "#x", canonical + "1",
		"https://www.bilibili.com.evil.example/video/av" + aid, "https://www.bilibili.com:443/video/av" + aid,
		"https://user@www.bilibili.com/video/av" + aid, "https://www.bilibili.com/video/BV135WM6uE4A",
	} {
		if ExactBilibiliWorkURL(aid, raw) {
			t.Fatal("unbound URL accepted", raw)
		}
	}
	when := time.Now().Add(-time.Minute)
	r := ResultResolution{AttemptID: 1, ExpectedRevision: 2, Conclusion: StatusPublished, PlatformSubmissionID: aid, PlatformWorkID: aid, TargetURL: canonical, WorkURL: canonical, PublishedAt: &when, Evidence: VerificationEvidence{PlatformUID: "u", Reason: "原稿已公开", Basis: "已实读原账号原稿件并核实匿名公开可播放", SceneClosed: true, WorkMatched: true, PubliclyAccessible: true}}
	if !r.ValidFor("bilibili") || r.Valid() {
		t.Fatal("platform URL rules were mixed")
	}
	r.PlatformSubmissionID = "117590282275873"
	r.TargetURL = ""
	if r.ValidFor("bilibili") {
		t.Fatal("another submission's public work was accepted")
	}
	r.PlatformSubmissionID = aid
	r.Conclusion = StatusReviewing
	r.PlatformWorkID = ""
	r.WorkURL = ""
	r.PublishedAt = nil
	if !r.ValidFor("bilibili") || r.ValidFor("xiaohongshu") {
		t.Fatal("incorrect reviewing platform validation")
	}
}
