package service

import (
	"context"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"testing"
	"time"

	filemodel "github.com/wt-media/wt-media-cloud/internal/modules/files/model"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/image/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/image/repository"
)

type memoryAsset struct{ rows map[uint64]model.ImageAsset }

func (m *memoryAsset) ByFile(id int64) (model.ImageAsset, error) {
	for _, v := range m.rows {
		if v.FileID == id {
			return v, nil
		}
	}
	return model.ImageAsset{}, ErrNotFound
}
func (m *memoryAsset) Register(v model.ImageAsset) (model.ImageAsset, error) {
	if _, e := m.ByFile(v.FileID); e == nil {
		return v, ErrConflict
	}
	v.ID = uint64(len(m.rows) + 1)
	m.rows[v.ID] = v
	return v, nil
}
func (m *memoryAsset) Get(id uint64) (model.ImageAsset, error) {
	v, ok := m.rows[id]
	if !ok {
		return v, ErrNotFound
	}
	return v, nil
}
func (m *memoryAsset) Update(v model.ImageAsset, rev uint64) (model.ImageAsset, error) {
	old, e := m.Get(v.ID)
	if e != nil {
		return v, e
	}
	if old.Revision != rev {
		return v, ErrConflict
	}
	v.Revision = rev + 1
	m.rows[v.ID] = v
	return v, nil
}
func (m *memoryAsset) List(sc model.Scope, f repository.Filter) ([]model.ImageAsset, int64, error) {
	out := []model.ImageAsset{}
	for _, v := range m.rows {
		if !sc.All && sc.OwnerID != v.CreatedBy && sc.TeamID != v.TeamID {
			continue
		}
		if f.Enabled != nil && v.Enabled != *f.Enabled {
			continue
		}
		if f.Usage == "cover" && !v.AllowAsCover || f.Usage == "background" && !v.AllowAsBackground {
			continue
		}
		if f.CategoryMode == "general" && len(v.CategoryIDs) != 0 {
			continue
		}
		if f.CategoryMode == "exact" {
			found := false
			for _, id := range v.CategoryIDs {
				found = found || id == f.CategoryID
			}
			if !found {
				continue
			}
		}
		out = append(out, v)
	}
	return out, int64(len(out)), nil
}
func assetFixture() (*Service, *memoryAsset, identity.PublicUser) {
	r := &memoryAsset{rows: map[uint64]model.ImageAsset{}}
	team := identity.TeamID(10)
	a := identity.PublicUser{ID: 1, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, CategoryIDs: []identity.CategoryID{2}}
	now := time.Now()
	ft := int64(10)
	s := &Service{Repo: r, ReadFile: func(context.Context, identity.PublicUser, int64) (filemodel.File, error) {
		return filemodel.File{ID: 7, OwnerID: 1, SizeBytes: 128, TeamID: &ft, Type: "image", UploadedAt: &now, MetadataJSON: json.RawMessage(`{"width":640,"height":360,"format":"png","source":"upload"}`)}, nil
	}, CategoryAllowed: func(a identity.PublicUser, id int64) (bool, error) { return id == 2, nil }}
	return s, r, a
}
func assetInput() RegisterInput {
	return RegisterInput{FileID: 7, Name: "封面", AllowAsCover: true, CategoryIDs: []string{"2"}}
}
func TestRegistrationRejectsEmptyUsageAndInaccessibleFile(t *testing.T) {
	s, r, a := assetFixture()
	in := assetInput()
	in.AllowAsCover = false
	if _, e := s.RegisterImageAsset(context.Background(), a, in); !errors.Is(e, ErrInvalid) {
		t.Fatalf("empty usage: %v", e)
	}
	in = assetInput()
	a.ID = 9
	if _, e := s.RegisterImageAsset(context.Background(), a, in); !errors.Is(e, ErrNotFound) {
		t.Fatalf("other owner's upload: %v", e)
	}
	if len(r.rows) != 0 {
		t.Fatal("invalid registration persisted")
	}
}
func TestRegistrationReplaysSameFileButRejectsChangedInput(t *testing.T) {
	s, _, a := assetFixture()
	in := assetInput()
	first, e := s.RegisterImageAsset(context.Background(), a, in)
	if e != nil {
		t.Fatal(e)
	}
	again, e := s.RegisterImageAsset(context.Background(), a, in)
	if e != nil || again.ID != first.ID || again.Revision != 1 || !again.Enabled {
		t.Fatalf("replay %+v %v", again, e)
	}
	in.Name = "changed"
	if _, e = s.RegisterImageAsset(context.Background(), a, in); !errors.Is(e, ErrConflict) {
		t.Fatalf("changed registration: %v", e)
	}
}
func TestReadAndEditEnforceOwnerTeamCategoryAndRevision(t *testing.T) {
	s, _, a := assetFixture()
	v, e := s.RegisterImageAsset(context.Background(), a, assetInput())
	if e != nil {
		t.Fatal(e)
	}
	other := a
	other.ID = 2
	if _, e = s.GetImageAsset(other, v.ID); !errors.Is(e, ErrNotFound) {
		t.Fatalf("other owner read: %v", e)
	}
	other.Role = identity.RoleSeniorOperator
	if _, e = s.GetImageAsset(other, v.ID); e != nil {
		t.Fatalf("same team senior: %v", e)
	}
	other.CategoryIDs = nil
	if _, e = s.GetImageAsset(other, v.ID); !errors.Is(e, ErrNotFound) {
		t.Fatalf("category scope: %v", e)
	}
	yes := false
	disabled, e := s.SetImageAssetUsage(a, v.ID, UsageInput{ExpectedRevision: 1, Enabled: &yes})
	if e != nil || disabled.Enabled {
		t.Fatalf("disable: %+v %v", disabled, e)
	}
	if _, e = s.SetImageAssetUsage(a, v.ID, UsageInput{ExpectedRevision: 1, Enabled: &yes}); !errors.Is(e, ErrConflict) {
		t.Fatalf("stale update: %v", e)
	}
	selected, e := s.SelectImageAsset(a, SelectInput{Usage: "cover", CategoryID: "2"})
	if e != nil || selected != nil {
		t.Fatalf("disabled selected: %+v %v", selected, e)
	}
}
func TestSelectionPrefersCategoryThenGeneralAndNeverAnotherCategory(t *testing.T) {
	s, r, a := assetFixture()
	v, e := s.RegisterImageAsset(context.Background(), a, assetInput())
	if e != nil {
		t.Fatal(e)
	}
	general := v
	general.ID = 2
	general.FileID = 8
	general.CategoryIDs = []string{}
	r.rows[2] = general
	selected, e := s.SelectImageAsset(a, SelectInput{Usage: "cover", CategoryID: "2"})
	if e != nil || selected == nil || selected.ID != v.ID {
		t.Fatalf("category priority: %+v %v", selected, e)
	}
	delete(r.rows, 1)
	selected, e = s.SelectImageAsset(a, SelectInput{Usage: "cover", CategoryID: "2"})
	if e != nil || selected == nil || selected.ID != 2 {
		t.Fatalf("general fallback: %+v %v", selected, e)
	}
	delete(r.rows, 2)
	other := v
	other.ID = 3
	other.CategoryIDs = []string{"9"}
	r.rows[3] = other
	selected, e = s.SelectImageAsset(a, SelectInput{Usage: "cover", CategoryID: "2"})
	if e != nil || selected != nil {
		t.Fatalf("unrelated category fallback: %+v %v", selected, e)
	}
}

func TestCategoryLookupFailureIsPreservedWithoutPersistingAsset(t *testing.T) {
	s, r, a := assetFixture()
	lookupErr := errors.New("category database unavailable")
	s.CategoryAllowed = func(identity.PublicUser, int64) (bool, error) { return false, lookupErr }
	if _, e := s.RegisterImageAsset(context.Background(), a, assetInput()); !errors.Is(e, lookupErr) {
		t.Fatalf("lookup failure hidden: %v", e)
	}
	if _, e := s.SelectImageAsset(a, SelectInput{Usage: "cover", CategoryID: "2"}); !errors.Is(e, lookupErr) {
		t.Fatalf("selection lookup failure hidden: %v", e)
	}
	if len(r.rows) != 0 {
		t.Fatal("failed lookup persisted an asset")
	}
}

func TestRegistrationRejectsUnverifiedOrCorruptImage(t *testing.T) {
	for _, kind := range []string{"size", "metadata", "dimensions", "unfinished", "deleted", "video"} {
		t.Run(kind, func(t *testing.T) {
			s, r, a := assetFixture()
			read := s.ReadFile
			s.ReadFile = func(ctx context.Context, a identity.PublicUser, id int64) (filemodel.File, error) {
				f, e := read(ctx, a, id)
				switch kind {
				case "size":
					f.SizeBytes = 0
				case "metadata":
					f.MetadataJSON = nil
				case "dimensions":
					f.MetadataJSON = []byte(`{"width":0,"height":360}`)
				case "unfinished":
					f.UploadedAt = nil
				case "deleted":
					now := time.Now()
					f.DeletedAt = &now
				case "video":
					f.Type = "video"
				}
				return f, e
			}
			if _, e := s.RegisterImageAsset(context.Background(), a, assetInput()); !errors.Is(e, ErrInvalid) {
				t.Fatalf("invalid image accepted: %v", e)
			}
			if len(r.rows) != 0 {
				t.Fatal("invalid image persisted")
			}
		})
	}
}

func (m *memoryAsset) Delete(id, revision uint64, check func(*gorm.DB, int64) error) error {
	v, e := m.Get(id)
	if e != nil {
		return e
	}
	if v.Revision != revision {
		return ErrConflict
	}
	if check == nil {
		return errors.New("missing reference check")
	}
	if e = check(nil, v.FileID); e != nil {
		return e
	}
	delete(m.rows, id)
	return nil
}

func TestImageBatchKeepsFailedItemAndAppliesIndependentChanges(t *testing.T) {
	s, r, a := assetFixture()
	v, e := s.RegisterImageAsset(context.Background(), a, RegisterInput{FileID: 7, Name: "图片", AllowAsCover: true, CategoryIDs: []string{}})
	if e != nil {
		t.Fatal(e)
	}
	other := v
	other.ID = 2
	other.FileID = 8
	r.rows[2] = other
	result, e := s.BatchImageAsset(a, BatchInput{Operation: "disable", Items: []BatchItem{{ID: v.ID, ExpectedRevision: v.Revision + 1}, {ID: 2, ExpectedRevision: other.Revision}}})
	if e != nil || len(result) != 2 || result[0].Success || !result[1].Success || !r.rows[v.ID].Enabled || r.rows[2].Enabled {
		t.Fatalf("partial result %+v %v", result, e)
	}
	if _, e = s.BatchImageAsset(a, BatchInput{Operation: "enable", Items: []BatchItem{{ID: 2, ExpectedRevision: 2}, {ID: 2, ExpectedRevision: 2}}}); !errors.Is(e, ErrInvalid) {
		t.Fatal("duplicate batch IDs accepted")
	}
}
func TestPreviewArtworkCannotBeRegisteredAsPublicationImage(t *testing.T) {
	s, _, a := assetFixture()
	read := s.ReadFile
	s.ReadFile = func(ctx context.Context, a identity.PublicUser, id int64) (filemodel.File, error) {
		f, e := read(ctx, a, id)
		f.MetadataJSON = []byte(`{"width":640,"height":360,"format":"png","source":"render","preview":true}`)
		return f, e
	}
	if _, e := s.RegisterImageAsset(context.Background(), a, RegisterInput{FileID: 7, Name: "预览图", AllowAsCover: true, CategoryIDs: []string{}}); !errors.Is(e, ErrInvalid) {
		t.Fatal("preview registered as formal image")
	}
}
