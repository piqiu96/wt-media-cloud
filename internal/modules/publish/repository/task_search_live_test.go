package repository_test

import (
	"encoding/json"
	"strconv"
	"testing"

	identitymodel "github.com/wt-media/wt-media-cloud/internal/modules/identity/model"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	repo "github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
)

func TestPublicationSearchFindsBilibiliTitleAndPreservesDescriptionAndScope(t *testing.T) {
	s, batch, _ := testExecutableBatch(t, 2, 5, "auto_continue")
	rows, _, err := s.BatchTaskPage(batch.ID, repo.BatchTaskFilter{Page: 1, PageSize: 10})
	if err != nil || len(rows) != 2 {
		t.Fatalf("fixtures: %v %v", rows, err)
	}
	for i, row := range rows {
		content := row.Content
		content.Title = "仅标题中的跳拉"
		content.Description = "独立简介内容"
		platform := "bilibili"
		if i == 1 {
			platform = "baidu"
			content.Title = "百度旧标题字段"
			content.Description = "百度正常文案"
		}
		body, err := json.Marshal(content)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.DB.Model(&model.Task{}).Where("id=?", row.ID).Updates(map[string]any{"platform": platform, "content_json": string(body)}).Error; err != nil {
			t.Fatal(err)
		}
	}
	actor := identity.PublicUser{ID: 9, Role: identity.RoleOperator, CategoryIDs: []identitymodel.CategoryID{1}}
	for _, tc := range []struct {
		name, search string
		want         int64
	}{
		{"Bilibili title absent from description", "仅标题中的跳拉", 1},
		{"Bilibili optional description", "独立简介", 1},
		{"Baidu description", "百度正常文案", 1},
		{"Baidu does not reinterpret unused title", "百度旧标题字段", 0},
		{"exact task ID", strconv.FormatUint(rows[0].ID, 10), 1},
		{"unmatched input", "不存在的文案", 0},
		{"parameterized input", "' OR 1=1 --", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := repo.Filter{Actor: actor, Search: tc.search, Page: 1, PageSize: 10}
			got, total, err := s.List(f)
			if err != nil || total != tc.want || int64(len(got)) != tc.want {
				t.Errorf("task list: total=%d rows=%d want=%d err=%v", total, len(got), tc.want, err)
			}
			summary, err := s.Summary(f)
			if err != nil || summary["all"] != tc.want {
				t.Errorf("task summary: %v want=%d err=%v", summary, tc.want, err)
			}
			got, total, err = s.BatchTaskPage(batch.ID, repo.BatchTaskFilter{Search: tc.search, Page: 1, PageSize: 10})
			if err != nil || total != tc.want || int64(len(got)) != tc.want {
				t.Errorf("batch list: total=%d rows=%d want=%d err=%v", total, len(got), tc.want, err)
			}
		})
	}
	actor.ID = 10
	if got, total, err := s.List(repo.Filter{Actor: actor, Search: "仅标题中的跳拉", Page: 1, PageSize: 10}); err != nil || total != 0 || len(got) != 0 {
		t.Fatalf("title search escaped owner scope: total=%d rows=%d err=%v", total, len(got), err)
	}
	actor.ID, actor.CategoryIDs = 9, []identitymodel.CategoryID{2}
	if _, total, err := s.List(repo.Filter{Actor: actor, Search: "仅标题中的跳拉", Page: 1, PageSize: 10}); err != nil || total != 0 {
		t.Fatalf("title search escaped category scope: total=%d err=%v", total, err)
	}
}
