package repository_test

import (
	"encoding/json"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"testing"
	"time"
)

func TestPlanPlatformChangeClearsIncompatibleResourcesButTargetEditPreservesInput(t *testing.T) {
	for _, tc := range []struct {
		from, to string
		change   bool
	}{{"bilibili", "baidu", true}, {"baidu", "bilibili", true}, {"bilibili", "bilibili", false}} {
		t.Run(tc.from+"-"+tc.to, func(t *testing.T) {
			s := store(isolated(t))
			now := time.Now().UTC()
			p := model.Plan{TeamID: 7, OwnerID: 9, CreatedBy: 9, Name: "platform reset", DateStart: "2026-11-26", DateEnd: "2026-11-27", Goals: []model.PlanGoal{{Platform: tc.from, CategoryID: 1, TargetCount: 1}}, RequestKey: "platform-reset", RequestHash: "digest", CreatedAt: now, UpdatedAt: now}
			rows := []model.PlanStructureBatch{{RowKey: "one", Name: "first", Platform: tc.from, GroupOrder: 1, BatchOrder: 1, Targets: []model.BatchTarget{{CategoryID: 1, TargetCount: 1}}}}
			p, bs, e := s.CreatePlan(p, rows)
			if e != nil {
				t.Fatal(e)
			}
			config := model.BatchConfig{Candidates: []model.BatchCandidate{{RowKey: "old", Content: model.Content{Title: "B站旧标题", Bilibili: &model.BilibiliContent{Statement: "内容含营销信息"}, SpaceCoverFileID: 1, Confirmed: true}}}}
			config.OutputIDs = []string{"42"}
			readiness := model.BatchReadiness{Ready: true, CoverResources: []model.BatchCoverResource{{RowKey: "old", SpaceCoverResourcePath: "old-reference"}}}
			body, _ := json.Marshal(config)
			ready, _ := json.Marshal(readiness)
			if e = s.DB.Model(&model.Batch{}).Where("id=?", bs[0].ID).Updates(map[string]any{"config_json": string(body), "readiness_json": string(ready), "candidates_count": 1, "configuration_status": model.BatchReady, "plan_configured_at": now}).Error; e != nil {
				t.Fatal(e)
			}
			p.Goals[0].Platform = tc.to
			rows[0].Platform = tc.to
			rows[0].BatchID = &bs[0].ID
			if !tc.change {
				p.Goals[0].TargetCount = 2
				rows[0].Targets[0].TargetCount = 2
			}
			if _, _, e = s.EditPlanStructure(p, p.Revision, rows); e != nil {
				t.Fatal(e)
			}
			b, e := s.GetBatch(bs[0].ID)
			if e != nil {
				t.Fatal(e)
			}
			if b.ConfigurationStatus != model.BatchIncomplete || b.PlanConfiguredAt != nil || b.Readiness.Ready {
				t.Fatalf("stale readiness: %+v", b)
			}
			if tc.change {
				if len(b.Config.Candidates) != 0 || len(b.Config.OutputIDs) != 0 || len(b.Config.Accounts) != 0 || b.CandidatesCount != 0 || len(b.Readiness.CoverResources) != 0 {
					t.Fatalf("incompatible input survived: %+v", b)
				}
				if len(b.Readiness.Shortfalls) != 1 || b.Readiness.Shortfalls[0].Shortfall != 1 {
					t.Fatalf("wrong new-platform shortfall: %+v", b.Readiness)
				}
			} else if len(b.Config.Candidates) != 1 || b.Config.Candidates[0].Content.Confirmed || b.Config.Candidates[0].Content.Title != "B站旧标题" {
				t.Fatal("target-only edit lost reusable input")
			}
			var tasks int64
			if e = s.DB.Model(&model.Task{}).Count(&tasks).Error; e != nil || tasks != 0 {
				t.Fatalf("structure edit created tasks: %d %v", tasks, e)
			}
		})
	}
}
