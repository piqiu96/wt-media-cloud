package model

import (
	"strings"
	"unicode/utf8"
)

type VerificationEvidence struct {
	PlatformUID       string `json:"platform_uid"`
	Reason            string `json:"reason"`
	Basis             string `json:"basis"`
	SceneClosed       bool   `json:"scene_closed"`
	NoPublishableWork bool   `json:"no_publishable_work"`
}
type FailureResolution struct {
	AttemptID        uint64               `json:"attempt_id,string"`
	ExpectedRevision uint64               `json:"expected_revision,string"`
	Conclusion       Status               `json:"conclusion"`
	Evidence         VerificationEvidence `json:"evidence"`
}

func (r FailureResolution) Valid() bool {
	e := r.Evidence
	return r.AttemptID > 0 && r.ExpectedRevision > 0 && (r.Conclusion == StatusFailed || r.Conclusion == StatusUncertain) && strings.TrimSpace(e.PlatformUID) != "" && len(e.PlatformUID) <= 255 && strings.TrimSpace(e.Reason) != "" && utf8.RuneCountInString(e.Reason) <= 200 && utf8.RuneCountInString(strings.TrimSpace(e.Basis)) >= 10 && utf8.RuneCountInString(e.Basis) <= 2000 && (r.Conclusion != StatusFailed || e.SceneClosed && e.NoPublishableWork)
}

type OriginalAttemptState struct {
	AttemptID      uint64  `json:"attempt_id,string"`
	ResultHash     *string `json:"result_hash"`
	SceneResolved  bool    `json:"scene_resolved"`
	ResolvedStatus Status  `json:"resolved_status"`
}
