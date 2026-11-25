package model

import (
	"encoding/hex"
	"strings"
	"unicode/utf8"
)

// These checks depend only on the original package, never today's resources.
func ValidPlanStartCommand(in StartPlanInput) bool {
	if !validCommandKey(in.RequestKey, 128) || in.ExpectedRevision == 0 || in.FirstBatchID == 0 || len(in.BatchConfirmations) == 0 || len(in.BatchConfirmations) > 1000 {
		return false
	}
	batches := map[uint64]bool{}
	first := false
	for _, c := range in.BatchConfirmations {
		if c.BatchID == 0 || c.ExpectedRevision == 0 || batches[c.BatchID] || len(c.IncludedRowKeys) == 0 || len(c.IncludedRowKeys) > 1000 || c.ExcludedRowKeys == nil || len(c.ExcludedRowKeys) > 1000 || len(c.ExpectedShortfalls) == 0 || len(c.ExpectedShortfalls) > 1000 {
			return false
		}
		batches[c.BatchID] = true
		first = first || c.BatchID == in.FirstBatchID
		rows := map[string]bool{}
		for _, keys := range [][]string{c.IncludedRowKeys, c.ExcludedRowKeys} {
			for _, key := range keys {
				if !validCommandKey(key, 64) || rows[key] {
					return false
				}
				rows[key] = true
			}
		}
		categories := map[int64]bool{}
		for _, g := range c.ExpectedShortfalls {
			if g.CategoryID <= 0 || g.Shortfall < 0 || categories[g.CategoryID] || g.Shortfall > 0 && !c.AllowShortfall {
				return false
			}
			categories[g.CategoryID] = true
		}
	}
	return first
}
func ValidPlanAdvanceCommand(in PlanAdvanceInput, automatic bool) bool {
	if !validCommandKey(in.RequestKey, 128) || in.ExpectedRevision == 0 || in.CurrentBatchID == 0 || in.NextBatchID == 0 || in.CurrentBatchID == in.NextBatchID || len(in.ExpectedBasisFingerprint) != 64 || strings.ToLower(in.ExpectedBasisFingerprint) != in.ExpectedBasisFingerprint {
		return false
	}
	if _, e := hex.DecodeString(in.ExpectedBasisFingerprint); e != nil {
		return false
	}
	return automatic || (in.Decision == "continue" || in.Decision == "skip") && strings.TrimSpace(in.Reason) != "" && utf8.RuneCountInString(in.Reason) <= 200
}
func validCommandKey(key string, limit int) bool {
	return strings.TrimSpace(key) == key && key != "" && utf8.ValidString(key) && utf8.RuneCountInString(key) <= limit
}
