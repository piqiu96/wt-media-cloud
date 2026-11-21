package model

import (
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

type VerificationEvidence struct {
	PlatformUID        string `json:"platform_uid"`
	Reason             string `json:"reason"`
	Basis              string `json:"basis"`
	SceneClosed        bool   `json:"scene_closed"`
	WorkMatched        bool   `json:"work_matched"`
	PubliclyAccessible bool   `json:"publicly_accessible"`
	NoPublishableWork  bool   `json:"no_publishable_work"`
}
type ResultResolution struct {
	PlatformSubmissionID string               `json:"platform_submission_id,omitempty"`
	TargetURL            string               `json:"target_url,omitempty"`
	PlatformWorkID       string               `json:"platform_work_id,omitempty"`
	WorkURL              string               `json:"work_url,omitempty"`
	PublishedAt          *time.Time           `json:"published_at,omitempty"`
	AttemptID            uint64               `json:"attempt_id,string"`
	ExpectedRevision     uint64               `json:"expected_revision,string"`
	Conclusion           Status               `json:"conclusion"`
	Evidence             VerificationEvidence `json:"evidence"`
}

func (r ResultResolution) Valid() bool {
	e := r.Evidence
	if r.AttemptID == 0 || r.ExpectedRevision == 0 || strings.TrimSpace(e.PlatformUID) == "" || len(e.PlatformUID) > 255 || strings.TrimSpace(e.Reason) == "" || utf8.RuneCountInString(e.Reason) > 200 || utf8.RuneCountInString(strings.TrimSpace(e.Basis)) < 10 || utf8.RuneCountInString(e.Basis) > 2000 {
		return false
	}
	switch r.Conclusion {
	case StatusFailed:
		return e.SceneClosed && e.NoPublishableWork && r.PlatformSubmissionID == "" && r.TargetURL == "" && r.PlatformWorkID == "" && r.WorkURL == "" && r.PublishedAt == nil
	case StatusUncertain:
		return r.PlatformSubmissionID == "" && r.TargetURL == "" && r.PlatformWorkID == "" && r.WorkURL == "" && r.PublishedAt == nil
	case StatusReviewing:
		return e.SceneClosed && e.WorkMatched && decimalIdentity(r.PlatformSubmissionID) && (r.TargetURL == "" || ExactBaiduTargetURL(r.PlatformSubmissionID, r.TargetURL)) && r.PlatformWorkID == "" && r.WorkURL == "" && r.PublishedAt == nil && !e.NoPublishableWork
	case StatusPublished:
		return e.SceneClosed && e.WorkMatched && e.PubliclyAccessible && !e.NoPublishableWork && decimalIdentity(r.PlatformSubmissionID) && (r.TargetURL == "" || ExactBaiduTargetURL(r.PlatformSubmissionID, r.TargetURL)) && ExactBaiduWorkURL(r.PlatformWorkID, r.WorkURL) && r.PublishedAt != nil && !r.PublishedAt.IsZero() && !r.PublishedAt.After(time.Now())
	}
	return false
}

type OriginalAttemptState struct {
	AttemptID      uint64  `json:"attempt_id,string"`
	ResultHash     *string `json:"result_hash"`
	SceneResolved  bool    `json:"scene_resolved"`
	ResolvedStatus Status  `json:"resolved_status"`
}

func decimalIdentity(id string) bool {
	if len(id) == 0 || len(id) > 255 || id[0] == '0' {
		return false
	}
	for _, c := range id {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// URL shape binds the supplied identity; it never proves public availability.
func ExactBaiduWorkURL(id, raw string) bool {
	if !decimalIdentity(id) {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || len(raw) > 2048 || u.Scheme != "https" || u.Host != "mbd.baidu.com" || u.User != nil || u.Fragment != "" || u.Path != "/newspage/data/videolanding" {
		return false
	}
	q, err := url.ParseQuery(u.RawQuery)
	return err == nil && len(q) == 1 && len(q["nid"]) == 1 && q.Get("nid") == "sv_"+id
}
func ExactBaiduTargetURL(id, raw string) bool {
	if !decimalIdentity(id) {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || len(raw) > 2048 || u.Scheme != "https" || u.Host != "baijiahao.baidu.com" || u.User != nil || u.Fragment != "" || u.Path != "/builder/preview/s" {
		return false
	}
	q, err := url.ParseQuery(u.RawQuery)
	return err == nil && len(q) == 1 && len(q["id"]) == 1 && q.Get("id") == id
}
