package model

import (
	"strings"
	"testing"
)

func TestFailureResolutionRequiresActualEvidenceAndSafeDisposition(t *testing.T) {
	good := FailureResolution{AttemptID: 9, ExpectedRevision: 3, Conclusion: "failed", Evidence: VerificationEvidence{PlatformUID: "original-owner", Reason: "已核对", Basis: "原账号稿件及原执行进程均已核实无待发布内容", SceneClosed: true, NoPublishableWork: true}}
	if !good.Valid() {
		t.Fatal("complete verification rejected")
	}
	cases := map[string]func(*FailureResolution){
		"missing attempt":                  func(v *FailureResolution) { v.AttemptID = 0 },
		"missing revision":                 func(v *FailureResolution) { v.ExpectedRevision = 0 },
		"public success outside this flow": func(v *FailureResolution) { v.Conclusion = "published" },
		"missing owner":                    func(v *FailureResolution) { v.Evidence.PlatformUID = " " },
		"missing reason":                   func(v *FailureResolution) { v.Evidence.Reason = " " },
		"short basis":                      func(v *FailureResolution) { v.Evidence.Basis = " 无依据 " },
		"oversize reason":                  func(v *FailureResolution) { v.Evidence.Reason = strings.Repeat("因", 201) },
		"oversize basis":                   func(v *FailureResolution) { v.Evidence.Basis = strings.Repeat("据", 2001) },
		"open scene":                       func(v *FailureResolution) { v.Evidence.SceneClosed = false },
		"publishable work not excluded":    func(v *FailureResolution) { v.Evidence.NoPublishableWork = false },
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
