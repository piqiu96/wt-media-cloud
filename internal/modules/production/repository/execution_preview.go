package repository

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"gorm.io/gorm"
)

var ErrComposePreviewChanged = errors.New("选材结果已变化，请重新预览")
var ErrComposeConfigInvalid = errors.New("策略成片设置无效，请检查模板参数")

var ErrComposeAlreadyRunning = errors.New("策略正在执行，请查看进度")

type ComposePreviewMaterial struct {
	MaterialID int64                            `json:"material_id"`
	Title      string                           `json:"title"`
	Selected   bool                             `json:"selected"`
	Reason     string                           `json:"reason"`
	Priority   int                              `json:"priority,omitempty"`
	Conditions []model.ComposeConditionDecision `json:"conditions,omitempty"`
}
type ComposeExecutionPreview struct {
	Strategy             model.ComposeStrategy        `json:"strategy"`
	SelectedCount        int                          `json:"selected_count"`
	EstimatedOutputCount int                          `json:"estimated_output_count"`
	SingleLimit          int                          `json:"single_limit"`
	DailyRemaining       *int                         `json:"daily_remaining"`
	Settings             []model.ComposeOutputSetting `json:"settings"`
	Materials            []ComposePreviewMaterial     `json:"materials"`
	ActiveBatch          *model.ComposeTaskBatch      `json:"active_batch"`
	PreviewToken         string                       `json:"preview_token"`
}

func (ComposeScheduleStore) Preview(strategy model.ComposeStrategy, now time.Time) (ComposeExecutionPreview, error) {
	return composeExecutionPreview(database.DB(), strategy, now, true)
}
func composeExecutionPreview(db *gorm.DB, strategy model.ComposeStrategy, now time.Time, immediate bool) (ComposeExecutionPreview, error) {
	if len(strategy.EffectiveOutputSettings()) == 0 {
		return ComposeExecutionPreview{}, ErrComposeConfigInvalid
	}
	p := ComposeExecutionPreview{Strategy: strategy, SingleLimit: strategy.SingleLimit, Settings: strategy.EffectiveOutputSettings(), Materials: []ComposePreviewMaterial{}}
	if immediate {
		var batchIDs []int64
		if err := db.Raw(`SELECT DISTINCT batch_id FROM compose_tasks WHERE compose_strategy_id=? AND status IN ('pending','running') AND batch_id IS NOT NULL ORDER BY batch_id DESC LIMIT 1`, strategy.ID).Scan(&batchIDs).Error; err != nil {
			return p, err
		}
		if len(batchIDs) > 0 {
			b, found, err := findComposeTaskBatch(db, batchIDs[0])
			if err != nil {
				return p, err
			}
			if found {
				p.ActiveBatch = &b
			}
			return p, nil
		}
	}
	candidates, err := listComposeScheduleCandidates(db, strategy)
	if err != nil {
		return p, err
	}
	remaining := strategy.SingleLimit
	if strategy.ExecutionMode == model.ComposeExecutionScheduled {
		zone := "Asia/Shanghai"
		if strategy.Schedule != nil {
			zone = strategy.Schedule.Timezone
		}
		loc, err := time.LoadLocation(zone)
		if err != nil {
			return p, err
		}
		n := now.In(loc)
		start := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, loc)
		var used int
		if err := db.Raw(`SELECT COUNT(DISTINCT material_id) FROM compose_tasks WHERE compose_strategy_id=? AND schedule_slot IS NOT NULL AND created_at>=? AND created_at<?`, strategy.ID, start.UTC(), start.AddDate(0, 0, 1).UTC()).Row().Scan(&used); err != nil {
			return p, err
		}
		r := strategy.DailyLimit - used
		if r < 0 {
			r = 0
		}
		p.DailyRemaining = &r
		if remaining > r {
			remaining = r
		}
	}
	available, err := composeOperatorAvailable(db, strategy)
	if err != nil {
		return p, err
	}
	var templateStatus string
	if err := db.Raw(`SELECT status FROM compose_templates WHERE id=? AND version=?`, strategy.TemplateID, strategy.TemplateVersion).Row().Scan(&templateStatus); err != nil {
		return p, err
	}
	// All rows remain visible in preview with a precise decision, while selection
	// follows rule priority then publication time/ID within that rule.
	for _, c := range candidates {
		d := model.EvaluateComposeRules(strategy.PriorityRules, c.facts(), now)
		row := ComposePreviewMaterial{MaterialID: c.MaterialID, Title: c.Title, Reason: d.Reason, Priority: d.Priority, Conditions: d.Conditions}
		reason, err := composeCandidateSkipReason(db, strategy, c, 0, 0)
		if err != nil {
			return p, err
		}
		if !available {
			reason = "operator_unavailable"
		} else if templateStatus != "active" {
			reason = "template_unavailable"
		}
		if reason != "" {
			row.Reason = reason
		} else {
			row.Selected = d.Eligible
		}
		p.Materials = append(p.Materials, row)
	}
	for priority := 1; priority <= len(strategy.PriorityRules); priority++ {
		for i := range p.Materials {
			row := &p.Materials[i]
			if !row.Selected || row.Priority != priority {
				continue
			}
			if p.SelectedCount >= remaining {
				row.Selected = false
				row.Reason = "per_run_limit"
				if p.DailyRemaining != nil && p.SelectedCount >= *p.DailyRemaining {
					row.Reason = "daily_limit"
				}
				continue
			}
			p.SelectedCount++
		}
	}
	for _, s := range p.Settings {
		p.EstimatedOutputCount += p.SelectedCount * s.Copies
	}
	// A fingerprint is not an authorization token. It detects stale reviewed facts;
	// confirmation repeats this read while holding the strategy command lock.
	hashInput := p
	hashInput.Strategy.OperatorName = ""
	payload, _ := json.Marshal(hashInput)
	digest := sha256.Sum256(payload)
	p.PreviewToken = hex.EncodeToString(digest[:])
	return p, nil
}
