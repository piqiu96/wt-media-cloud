package repository_test

import (
	"fmt"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	repo "github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
	"reflect"
	"sort"
	"testing"
)

func TestLiveOutputOccupancyAcrossHistoryOwnersStatesAndPlatforms(t *testing.T) {
	db := isolated(t)
	s := store(db)
	statuses := []model.Status{model.StatusPending, model.StatusQueued, model.StatusRunning, model.StatusAwaitingHuman, model.StatusUncertain, model.StatusReviewing, model.StatusFailed, model.StatusPublished, model.StatusCancelled, model.StatusAbandoned}
	ids := []int64{}
	for id := int64(120); id <= 131; id++ {
		if err := db.Exec("INSERT INTO composite_outputs(id) VALUES(?)", id).Error; err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	for i, status := range statuses {
		v := task(fmt.Sprint("old-", i), int64(120+i), 1)
		v.Status = status
		v.PlatformAccountUID = fmt.Sprint("old-uid-", i)
		v.OperatorID = 77
		v.TeamID = 99
		v.CategoryID = 88
		v.ID = uint64(i + 1)
		if err := db.Create(&v).Error; err != nil {
			t.Fatal(err)
		}
	}
	other := task("other-platform", 130, 1)
	other.Platform = "bilibili"
	other.ID = 50
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	actor := identity.PublicUser{ID: 9, Role: identity.RoleOperator, CategoryIDs: []identity.CategoryID{1}}
	if rows, _, err := s.List(repo.Filter{Actor: actor, Page: 8, PageSize: 1}); err != nil || len(rows) != 0 {
		t.Fatalf("fixture publication page: %v %v", rows, err)
	}
	occupied, err := s.OccupiedOutputIDs("baidu", ids)
	sort.Slice(occupied, func(i, j int) bool { return occupied[i] < occupied[j] })
	if err != nil || !reflect.DeepEqual(occupied, ids[:9]) {
		t.Fatalf("history/other owner occupancy lost: %v %v", occupied, err)
	}
	occupied, err = s.OccupiedOutputIDs("bilibili", []int64{129, 130, 131})
	if err != nil || !reflect.DeepEqual(occupied, []int64{130}) {
		t.Fatalf("cross-platform occupancy: %v %v", occupied, err)
	}
	occupied, err = s.OccupiedOutputIDs("baidu", []int64{129})
	if err != nil || len(occupied) != 0 {
		t.Fatalf("abandoned task must release its output: %v %v", occupied, err)
	}
	if _, err = s.Create(task("reused-abandoned-output", 129, 1)); err != nil {
		t.Fatal(err)
	}
	occupied, err = s.OccupiedOutputIDs("baidu", []int64{129})
	if err != nil || !reflect.DeepEqual(occupied, []int64{129}) {
		t.Fatalf("new task must hold the released output again: %v %v", occupied, err)
	}
	if _, err = s.Create(task("new-task-7", 131, 1)); err != nil {
		t.Fatal(err)
	}
	occupied, err = s.OccupiedOutputIDs("baidu", []int64{131})
	if err != nil || !reflect.DeepEqual(occupied, []int64{131}) {
		t.Fatalf("newly reserved output must stop being available: %v %v", occupied, err)
	}
}
