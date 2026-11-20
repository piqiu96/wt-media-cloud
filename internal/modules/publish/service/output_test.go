package service

import (
	"context"
	"encoding/json"
	"errors"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	outputmodel "github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	production "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"reflect"
	"testing"
)

type candidateTasks struct {
	*memoryTasks
	ids       []int64
	err       error
	called    bool
	platform  string
	requested []int64
}

func (r *candidateTasks) OccupiedOutputIDs(platform string, ids []int64) ([]int64, error) {
	r.called = true
	r.platform = platform
	r.requested = ids
	return r.ids, r.err
}
func (r *candidateTasks) List(Filter) ([]model.Task, int64, error) {
	panic("candidate occupancy must not read a publication page")
}
func TestOutputCandidatesUseProductionScopeAndGlobalOccupancy(t *testing.T) {
	for _, actor := range []identity.PublicUser{{Role: identity.RoleOperator}, {Role: identity.RoleSeniorOperator}, {Role: identity.RoleAdmin}} {
		role := actor.Role
		t.Run(string(role), func(t *testing.T) {
			s, a, _ := fixture()
			a.Role = role
			repo := &candidateTasks{memoryTasks: s.Repo.(*memoryTasks), ids: []int64{129, 130}}
			s.Repo = repo
			input := production.ListComposeOutputsInput{Page: 8, PageSize: 10, Search: "source", Platform: "baidu"}
			s.OutputPage = func(_ context.Context, actor identity.PublicUser, got production.ListComposeOutputsInput) (OutputPage, error) {
				if !reflect.DeepEqual(actor, a) || !reflect.DeepEqual(got, input) {
					t.Fatalf("production actor or filters changed: %+v %+v", actor, got)
				}
				return OutputPage{Items: []outputmodel.CompositeOutput{{ID: 129}, {ID: 130}, {ID: 131}}, Total: 83, Page: 8, PageSize: 10}, nil
			}
			page, err := s.OutputCandidates(context.Background(), a, input)
			if err != nil || page.Total != 83 || page.Page != 8 || len(page.Items) != 3 || !page.Items[0].Occupied || !page.Items[1].Occupied || page.Items[2].Occupied {
				t.Fatalf("page=%+v err=%v", page, err)
			}
			if repo.platform != "baidu" || !reflect.DeepEqual(repo.requested, []int64{129, 130, 131}) {
				t.Fatalf("wrong batch: %+v", repo)
			}
			raw, _ := json.Marshal(page)
			var body map[string]any
			_ = json.Unmarshal(raw, &body)
			for _, row := range body["items"].([]any) {
				if _, ok := row.(map[string]any)["task_id"]; ok {
					t.Fatal("occupying task identity leaked")
				}
			}
		})
	}
}
func TestOutputCandidatesFailClosedOnProviderOrOccupancyReadFailure(t *testing.T) {
	s, a, _ := fixture()
	repo := &candidateTasks{memoryTasks: s.Repo.(*memoryTasks)}
	s.Repo = repo
	in := production.ListComposeOutputsInput{Page: 1, PageSize: 10, Platform: "baidu"}
	s.OutputPage = func(context.Context, identity.PublicUser, production.ListComposeOutputsInput) (OutputPage, error) {
		return OutputPage{}, production.ErrComposeOutputForbidden
	}
	if _, err := s.OutputCandidates(context.Background(), a, in); !errors.Is(err, production.ErrComposeOutputForbidden) || repo.called {
		t.Fatalf("production denial bypassed: %v %+v", err, repo)
	}
	s.OutputPage = func(context.Context, identity.PublicUser, production.ListComposeOutputsInput) (OutputPage, error) {
		return OutputPage{Items: []outputmodel.CompositeOutput{{ID: 131}}}, nil
	}
	repo.err = errors.New("database unavailable")
	if page, err := s.OutputCandidates(context.Background(), a, in); !errors.Is(err, repo.err) || len(page.Items) > 0 {
		t.Fatalf("unknown occupancy shown as available: %+v %v", page, err)
	}
}
