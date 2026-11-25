package model

import (
	"strings"
	"testing"
)

func TestOriginalPlanCommandStaticValidationNeverUsesMutableFacts(t *testing.T) {
	start := StartPlanInput{RequestKey: "original", ExpectedRevision: 1, FirstBatchID: 2, BatchConfirmations: []PlanBatchConfirmation{{BatchID: 2, ExpectedRevision: 1, IncludedRowKeys: []string{"row"}, ExcludedRowKeys: []string{}, ExpectedShortfalls: []ExpectedShortfall{{CategoryID: 3}}}}}
	if !ValidPlanStartCommand(start) {
		t.Fatal("legal original")
	}
	for _, mutate := range []func(*StartPlanInput){func(v *StartPlanInput) { v.FirstBatchID = 9 }, func(v *StartPlanInput) { v.BatchConfirmations[0].ExcludedRowKeys = []string{"row"} }, func(v *StartPlanInput) { v.BatchConfirmations[0].IncludedRowKeys = []string{"row", "row"} }, func(v *StartPlanInput) {
		v.BatchConfirmations[0].ExpectedShortfalls = []ExpectedShortfall{{CategoryID: 3, Shortfall: -1}}
	}, func(v *StartPlanInput) {
		v.BatchConfirmations[0].ExpectedShortfalls = []ExpectedShortfall{{CategoryID: 3}, {CategoryID: 3}}
	}, func(v *StartPlanInput) { v.BatchConfirmations[0].ExcludedRowKeys = nil }, func(v *StartPlanInput) { v.BatchConfirmations = append(v.BatchConfirmations, v.BatchConfirmations[0]) }} {
		copy := start
		copy.BatchConfirmations = append([]PlanBatchConfirmation{}, start.BatchConfirmations...)
		mutate(&copy)
		if ValidPlanStartCommand(copy) {
			t.Fatalf("invalid accepted %+v", copy)
		}
	}
	advance := PlanAdvanceInput{RequestKey: "next", ExpectedRevision: 1, CurrentBatchID: 2, NextBatchID: 3, ExpectedBasisFingerprint: strings.Repeat("a", 64), Decision: "continue", Reason: "确认继续"}
	if !ValidPlanAdvanceCommand(advance, false) {
		t.Fatal("legal next")
	}
	for _, mutate := range []func(*PlanAdvanceInput){func(v *PlanAdvanceInput) { v.ExpectedBasisFingerprint = strings.Repeat("g", 64) }, func(v *PlanAdvanceInput) { v.NextBatchID = v.CurrentBatchID }, func(v *PlanAdvanceInput) { v.Decision = "automatic" }, func(v *PlanAdvanceInput) { v.Reason = "" }} {
		copy := advance
		mutate(&copy)
		if ValidPlanAdvanceCommand(copy, false) {
			t.Fatalf("invalid accepted %+v", copy)
		}
	}
}
