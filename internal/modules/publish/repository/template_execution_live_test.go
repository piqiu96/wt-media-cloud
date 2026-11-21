package repository_test

import (
	"encoding/json"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"testing"
	"time"
)

func TestLiveTemplateSnapshotComparedByValueAcrossExecutionReports(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(map[bool]string{false: "same_saved_cover", true: "changed_saved_cover"}[changed], func(t *testing.T) {
			s, makeTask := ownedStore(t)
			v := makeTask(0)
			v.Content.CoverRender = &model.CoverRender{TemplateID: 8, PublishedRevision: 2, Title: "原封面标题", RenderInputHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
			raw, _ := json.Marshal(v.Content)
			if err := s.DB.Exec("UPDATE publication_task SET content_json=? WHERE id=?", string(raw), v.ID).Error; err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC()
			v, err := s.Get(v.ID)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.Start(v, v.Revision, 9, now); err != nil {
				t.Fatal(err)
			}
			c, err := s.Claim(9, "template-cover-report", now)
			if err != nil || c == nil {
				t.Fatalf("claim: %v", err)
			}
			if changed {
				v.Content.CoverRender.Title = "改过封面标题"
				raw, _ = json.Marshal(v.Content)
				if err = s.DB.Exec("UPDATE publication_task SET content_json=? WHERE id=?", string(raw), v.ID).Error; err != nil {
					t.Fatal(err)
				}
			}
			for n, stage := range []model.AttemptStage{model.AttemptStagePreparing, model.AttemptStageReady, model.AttemptStageSubmitting} {
				in := c.Attempt.Input
				feedback, err := s.Report(c.Task, 9, model.ExecutionReport{AttemptID: c.Attempt.ID, Seq: uint64(n + 1), Stage: stage, PlatformUID: in.PlatformUID, BitProfileID: in.BitProfileID, VideoSHA256: in.VideoSHA256, CoverSHA256: in.CoverSHA256}, true, now)
				if err != nil {
					t.Fatal(err)
				}
				if changed {
					if !feedback.StopRequested || feedback.StopReason != model.ExecutionInputChanged || feedback.AllowSubmit {
						t.Fatalf("changed input continued: %+v", feedback)
					}
					break
				}
				if feedback.StopRequested || feedback.AllowSubmit != (stage == model.AttemptStageSubmitting) {
					t.Fatalf("same frozen cover rejected at %s: %+v", stage, feedback)
				}
			}
		})
	}
}
