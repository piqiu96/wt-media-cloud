package service

import (
	"context"
	"errors"
	filemodel "github.com/wt-media/wt-media-cloud/internal/modules/files/model"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	accounts "github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/service"
	outputmodel "github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	production "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"testing"
)

func fixture() (*Service, identity.PublicUser, CreateInput) {
	team := identity.TeamID(7)
	owner := int64(9)
	actor := identity.PublicUser{ID: 9, Status: identity.UserStatusEnabled, Role: identity.RoleOperator, TeamID: &team, CategoryIDs: []identity.CategoryID{1}}
	s := &Service{Repo: &memoryTasks{items: map[uint64]model.Task{}}, Account: func(identity.PublicUser, string) (accounts.Account, error) {
		return accounts.Account{ID: "12", UserID: 9, TeamID: &team, CategoryIDs: []identity.CategoryID{1}, Enabled: true, Platform: "baidu", PlatformAccountID: "real-uid", BrowserProfileID: "profile", IdentificationStatus: "identified", PlatformLoginStatus: "normal"}, nil
	}, Output: func(identity.PublicUser, int64) (production.ComposeOutputDetail, error) {
		return production.ComposeOutputDetail{Output: outputmodel.CompositeOutput{ID: 42, TeamID: 7, CategoryID: 1, OperatorID: &owner, PublishStatus: "pending_publish", Platforms: []string{"baidu"}, ObjectKey: "video", FileSizeBytes: 3, SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Video: outputmodel.ComposeOutputVideo{Status: "ready"}}}, nil
	}, File: func(context.Context, identity.PublicUser, int64) (filemodel.File, error) {
		return filemodel.File{ID: 5, Type: "image", ObjectKey: "cover", SizeBytes: 3, SHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}, nil
	}}
	return s, actor, CreateInput{RequestKey: "request-1", Platform: "baidu", CategoryID: 1, MediaAccountID: 12, CompositeOutputID: 42, Content: model.Content{Description: "真实作品描述", CoverFileID: 5, Declaration: "无需声明", Confirmed: true}}
}

type memoryTasks struct {
	items map[uint64]model.Task
	next  uint64
}

func (m *memoryTasks) ByRequest(actor int64, key string) (model.Task, error) {
	for _, v := range m.items {
		if v.CreatedBy == actor && v.RequestKey == key {
			return v, nil
		}
	}
	return model.Task{}, ErrNotFound
}
func (m *memoryTasks) Create(v model.Task) (model.Task, error) {
	for _, old := range m.items {
		if old.Platform == v.Platform && old.CompositeOutputID == v.CompositeOutputID {
			return model.Task{}, ErrOccupied
		}
	}
	m.next++
	v.ID = m.next
	m.items[v.ID] = v
	return v, nil
}
func (m *memoryTasks) Get(id uint64) (model.Task, error) {
	v, ok := m.items[id]
	if !ok {
		return v, ErrNotFound
	}
	return v, nil
}
func (m *memoryTasks) List(f Filter) ([]model.Task, int64, error) {
	out := []model.Task{}
	for _, v := range m.items {
		if CanRead(f.Actor, v) {
			out = append(out, v)
		}
	}
	return out, int64(len(out)), nil
}
func (m *memoryTasks) Summary(f Filter) (map[string]int64, error) { return map[string]int64{}, nil }
func (m *memoryTasks) Edit(v model.Task, revision uint64) (model.Task, error) {
	old := m.items[v.ID]
	if old.Revision != revision || old.Status != "pending" || old.CurrentAttemptID != nil {
		return old, ErrConflict
	}
	v.Revision = old.Revision + 1
	m.items[v.ID] = v
	return v, nil
}
func TestCreateIsPendingIdempotentAndOccupiesOutput(t *testing.T) {
	s, a, in := fixture()
	first, err := s.Create(context.Background(), a, in)
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != "pending" || first.AuthorizedAt != nil || first.OperatorID != 9 {
		t.Fatalf("unexpected execution: %+v", first)
	}
	again, err := s.Create(context.Background(), a, in)
	if err != nil || again.ID != first.ID {
		t.Fatalf("replay: %+v %v", again, err)
	}
	in.Content.Description = "changed"
	if _, err = s.Create(context.Background(), a, in); !errors.Is(err, ErrConflict) {
		t.Fatalf("same key changed input: %v", err)
	}
	in.RequestKey = "second"
	if _, err = s.Create(context.Background(), a, in); !errors.Is(err, ErrOccupied) {
		t.Fatalf("duplicate output: %v", err)
	}
}
func TestCreateRejectsUnavailableResourcesAndPlatform(t *testing.T) {
	for _, name := range []string{"wrong-platform", "unconfirmed", "missing-cover", "bad-declaration", "outside-category", "file-unavailable", "output-unavailable"} {
		t.Run(name, func(t *testing.T) {
			s, a, in := fixture()
			switch name {
			case "wrong-platform":
				in.Platform = "bilibili"
			case "unconfirmed":
				in.Content.Confirmed = false
			case "missing-cover":
				in.Content.CoverFileID = 0
			case "bad-declaration":
				in.Content.Declaration = "原创"
			case "outside-category":
				in.CategoryID = 2
			case "file-unavailable":
				s.File = func(context.Context, identity.PublicUser, int64) (filemodel.File, error) {
					return filemodel.File{}, errors.New("missing object")
				}
			case "output-unavailable":
				s.Output = func(identity.PublicUser, int64) (production.ComposeOutputDetail, error) {
					return production.ComposeOutputDetail{}, errors.New("not readable")
				}
			}
			if _, err := s.Create(context.Background(), a, in); err == nil {
				t.Fatal("invalid resource accepted")
			}
			if len(s.Repo.(*memoryTasks).items) != 0 {
				t.Fatal("invalid resource occupied output")
			}
		})
	}
}
func TestEditChecksPermissionRevisionAndPendingState(t *testing.T) {
	s, a, in := fixture()
	v, err := s.Create(context.Background(), a, in)
	if err != nil {
		t.Fatal(err)
	}
	other := a
	other.ID = 10
	if _, err = s.Get(other, v.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("other owner can see task")
	}
	content := in.Content
	content.Description = "新文案"
	edited, err := s.Edit(context.Background(), a, v.ID, EditInput{ExpectedRevision: 1, Content: content})
	if err != nil || edited.Revision != 2 || edited.MediaAccountID != v.MediaAccountID {
		t.Fatalf("edit: %+v %v", edited, err)
	}
	if _, err = s.Edit(context.Background(), a, v.ID, EditInput{ExpectedRevision: 1, Content: content}); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale edit: %v", err)
	}
	r := s.Repo.(*memoryTasks)
	v = r.items[v.ID]
	v.Status = "queued"
	r.items[v.ID] = v
	if _, err = s.Edit(context.Background(), a, v.ID, EditInput{ExpectedRevision: 2, Content: content}); !errors.Is(err, ErrConflict) {
		t.Fatalf("queued edit: %v", err)
	}
}

func TestBaiduPreparationNeedsNoFeatureSwitch(t *testing.T) {
	resources, actor, input := fixture()
	s := &Service{Repo: resources.Repo, Account: resources.Account, Output: resources.Output, File: resources.File}
	task, err := s.Create(context.Background(), actor, input)
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != "pending" || task.AuthorizedAt != nil || task.CurrentAttemptID != nil {
		t.Fatalf("create authorized execution: %+v", task)
	}
}

func TestCreateAndEditRejectRendererPreview(t *testing.T) {
	s, a, in := fixture()
	original := s.File
	preview := func(ctx context.Context, a identity.PublicUser, id int64) (filemodel.File, error) {
		f, e := original(ctx, a, id)
		f.MetadataJSON = []byte(`{"preview":true}`)
		return f, e
	}
	s.File = preview
	if _, err := s.Create(context.Background(), a, in); err == nil {
		t.Fatal("preview became formal task")
	}
	s.File = original
	task, err := s.Create(context.Background(), a, in)
	if err != nil {
		t.Fatal(err)
	}
	s.File = preview
	in.Content.CoverFileID++
	if _, err = s.Edit(context.Background(), a, task.ID, EditInput{ExpectedRevision: task.Revision, Content: in.Content}); err == nil {
		t.Fatal("preview replaced formal cover")
	}
}

func TestFindCreationByRequestKeepsCreatorAndResourceScope(t *testing.T) {
	s, actor, input := fixture()
	task, err := s.Create(context.Background(), actor, input)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.FindByRequest(actor, input.RequestKey)
	if err != nil || got.ID != task.ID {
		t.Fatalf("lookup: %+v %v", got, err)
	}
	other := actor
	other.ID = 10
	if _, err := s.FindByRequest(other, input.RequestKey); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other creator: %v", err)
	}
	actor.CategoryIDs = nil
	if _, err := s.FindByRequest(actor, input.RequestKey); !errors.Is(err, ErrNotFound) {
		t.Fatalf("revoked category: %v", err)
	}
}
