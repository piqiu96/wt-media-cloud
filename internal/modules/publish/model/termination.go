package model

import (
	"strings"
	"unicode/utf8"
)

// Abandonment records an explicit human decision after the original attempt's
// safe failure has already been established. It cannot resolve an unknown scene.
type Abandonment struct {
	AttemptID        uint64               `json:"attempt_id,string"`
	ExpectedRevision uint64               `json:"expected_revision,string"`
	Evidence         VerificationEvidence `json:"evidence"`
}

func (a Abandonment) Valid() bool {
	return (FailureResolution{AttemptID: a.AttemptID, ExpectedRevision: a.ExpectedRevision, Conclusion: StatusFailed, Evidence: a.Evidence}).Valid()
}

type Cancellation struct {
	ExpectedRevision uint64 `json:"expected_revision,string"`
	Reason           string `json:"reason"`
}

func (c Cancellation) Valid() bool {
	return c.ExpectedRevision > 0 && strings.TrimSpace(c.Reason) != "" && utf8.RuneCountInString(c.Reason) <= 200
}
