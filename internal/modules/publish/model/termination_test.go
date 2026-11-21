package model

import (
	"strings"
	"testing"
)

func TestCancellationRequiresRevisionAndHumanReason(t *testing.T) {
	for _, v := range []Cancellation{{}, {ExpectedRevision: 1}, {ExpectedRevision: 1, Reason: " \n "}, {ExpectedRevision: 1, Reason: strings.Repeat("因", 201)}} {
		if v.Valid() {
			t.Fatalf("invalid cancellation accepted: %+v", v)
		}
	}
	if !(Cancellation{ExpectedRevision: 1, Reason: strings.Repeat("因", 200)}).Valid() {
		t.Fatal("valid boundary rejected")
	}
}
