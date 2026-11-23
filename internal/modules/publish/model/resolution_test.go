package model

import (
	"strings"
	"testing"
	"time"
)

func TestResultResolutionRequiresActualEvidenceAndSafeDisposition(t *testing.T) {
	good := ResultResolution{AttemptID: 9, ExpectedRevision: 3, Conclusion: "failed", Evidence: VerificationEvidence{PlatformUID: "original-owner", Reason: "已核对", Basis: "原账号稿件及原执行进程均已核实无待发布内容", SceneClosed: true, NoPublishableWork: true}}
	if !good.Valid() {
		t.Fatal("complete verification rejected")
	}
	cases := map[string]func(*ResultResolution){
		"missing attempt":                  func(v *ResultResolution) { v.AttemptID = 0 },
		"missing revision":                 func(v *ResultResolution) { v.ExpectedRevision = 0 },
		"public success outside this flow": func(v *ResultResolution) { v.Conclusion = "published" },
		"missing owner":                    func(v *ResultResolution) { v.Evidence.PlatformUID = " " },
		"missing reason":                   func(v *ResultResolution) { v.Evidence.Reason = " " },
		"short basis":                      func(v *ResultResolution) { v.Evidence.Basis = " 无依据 " },
		"oversize reason":                  func(v *ResultResolution) { v.Evidence.Reason = strings.Repeat("因", 201) },
		"oversize basis":                   func(v *ResultResolution) { v.Evidence.Basis = strings.Repeat("据", 2001) },
		"open scene":                       func(v *ResultResolution) { v.Evidence.SceneClosed = false },
		"publishable work not excluded":    func(v *ResultResolution) { v.Evidence.NoPublishableWork = false },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			v := good
			change(&v)
			if v.Valid() {
				t.Fatal("unsafe verification accepted")
			}
		})
	}
	good.Conclusion = "uncertain"
	good.Evidence.SceneClosed = false
	good.Evidence.NoPublishableWork = false
	if !good.Valid() {
		t.Fatal("honest unresolved verification rejected")
	}
}

func TestPublishedResolutionRequiresExactWorkAndExplicitEvidence(t *testing.T) {
	now := time.Now().UTC().Add(-time.Hour)
	good := ResultResolution{AttemptID: 9, ExpectedRevision: 3, Conclusion: "published", PlatformSubmissionID: "1882371730190912814", PlatformWorkID: "7000395122085014122", WorkURL: "https://mbd.baidu.com/newspage/data/videolanding?nid=sv_7000395122085014122", PublishedAt: &now, Evidence: VerificationEvidence{PlatformUID: "6572476037", Reason: "已核实公开", Basis: "核对指定稿件跳转公开作品、原账号及视频一致，实际发布时间来自平台", SceneClosed: true, WorkMatched: true, PubliclyAccessible: true}}
	if !good.Valid() {
		t.Fatal("complete public evidence rejected")
	}
	for name, change := range map[string]func(*ResultResolution){
		"preview": func(v *ResultResolution) {
			v.WorkURL = "https://baijiahao.baidu.com/builder/preview/s?id=1882371730190912814"
		},
		"wrong work":          func(v *ResultResolution) { v.PlatformWorkID = "1882371730190912814" },
		"future":              func(v *ResultResolution) { x := time.Now().Add(time.Hour); v.PublishedAt = &x },
		"no public evidence":  func(v *ResultResolution) { v.Evidence.PubliclyAccessible = false },
		"no content evidence": func(v *ResultResolution) { v.Evidence.WorkMatched = false },
		"no actual time":      func(v *ResultResolution) { v.PublishedAt = nil },
	} {
		t.Run(name, func(t *testing.T) {
			v := good
			change(&v)
			if v.Valid() {
				t.Fatal("unsafe public conclusion accepted")
			}
		})
	}
	good.Conclusion = "reviewing"
	good.Evidence.NoPublishableWork = false
	good.PlatformWorkID = ""
	good.WorkURL = ""
	good.PublishedAt = nil
	good.Evidence.PubliclyAccessible = false
	if !good.Valid() {
		t.Fatal("exact receipt rejected")
	}
	good.TargetURL = "https://baijiahao.baidu.com/builder/preview/s?id=other"
	if good.Valid() {
		t.Fatal("foreign receipt URL accepted")
	}
}
