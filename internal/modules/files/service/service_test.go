package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/media"
	"github.com/wt-media/wt-media-cloud/internal/infra/storage"
	"github.com/wt-media/wt-media-cloud/internal/modules/files/model"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
)

type memoryFiles struct {
	records map[int64]model.File
	next    int64
}

func (m *memoryFiles) SetLocation(id int64, key string) error {
	f := m.records[id]
	f.ObjectKey = key
	m.records[id] = f
	return nil
}

func (m *memoryFiles) Insert(f *model.File) error {
	m.next++
	f.ID = m.next
	m.records[f.ID] = *f
	return nil
}
func (m *memoryFiles) Ready(f model.File) error { m.records[f.ID] = f; return nil }
func (m *memoryFiles) Find(id int64) (model.File, error) {
	f, ok := m.records[id]
	if !ok || f.UploadedAt == nil {
		return f, ErrNotFound
	}
	return f, nil
}
func (m *memoryFiles) FindToken(token string) (model.File, error) {
	for _, f := range m.records {
		if f.ResourceToken == token && f.UploadedAt != nil {
			return f, nil
		}
	}
	return model.File{}, ErrNotFound
}
func (m *memoryFiles) Rename(id int64, name string) error {
	f := m.records[id]
	f.Name = name
	m.records[id] = f
	return nil
}
func (m *memoryFiles) BeginDelete(id int64) error {
	f := m.records[id]
	if f.DeletedAt == nil {
		now := time.Now().UTC()
		f.DeletedAt = &now
	}
	m.records[id] = f
	return nil
}
func (m *memoryFiles) Delete(id int64) error { delete(m.records, id); return nil }
func (m *memoryFiles) List(scope model.Scope, kind, search string, page, size int) ([]model.File, int64, error) {
	var out []model.File
	for _, f := range m.records {
		if f.UploadedAt != nil && (scope.All || scope.OwnerID == f.OwnerID || (scope.TeamID > 0 && f.TeamID != nil && scope.TeamID == *f.TeamID)) {
			out = append(out, f)
		}
	}
	return out, int64(len(out)), nil
}

type memoryObjects struct {
	contents             map[string]string
	uploadErr, removeErr error
}

func (m *memoryObjects) Reference(k string) (string, error) {
	return (&url.URL{Scheme: "s3", Host: "wt-media", Path: "/" + k}).String(), nil
}
func (m *memoryObjects) Stat(_ context.Context, k string) (storage.ObjectInfo, error) {
	s, ok := m.contents[k]
	if !ok {
		return storage.ObjectInfo{}, storage.ErrObjectNotFound
	}
	return storage.ObjectInfo{Size: int64(len(s))}, nil
}
func (m *memoryObjects) Open(_ context.Context, k string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(m.contents[k])), nil
}
func (m *memoryObjects) PutImmutable(_ context.Context, k string, r io.Reader, n int64, ct, sha string) error {
	if m.uploadErr != nil {
		return m.uploadErr
	}
	b, _ := io.ReadAll(r)
	m.contents[k] = string(b)
	return nil
}
func (m *memoryObjects) Put(context.Context, string, io.Reader, int64, string) error {
	return errors.New("must use immutable upload")
}
func (m *memoryObjects) Copy(context.Context, string, string) error {
	return errors.New("unexpected copy")
}
func (m *memoryObjects) Remove(_ context.Context, k string) error {
	if m.removeErr != nil {
		return m.removeErr
	}
	delete(m.contents, k)
	return nil
}
func (m *memoryObjects) PresignGet(context.Context, string, time.Duration) (storage.Grant, error) {
	return storage.Grant{URL: "https://bucket.example/object"}, nil
}
func operator(id, team int64, role identity.Role) identity.PublicUser {
	t := identity.TeamID(team)
	return identity.PublicUser{ID: identity.UserID(id), Role: role, Status: identity.UserStatusEnabled, TeamID: &t}
}
func testService() (*Service, *memoryFiles, *memoryObjects) {
	r := &memoryFiles{records: map[int64]model.File{}}
	o := &memoryObjects{contents: map[string]string{}}
	return &Service{Repo: r, Objects: o, PublicBaseURL: "https://wt-media.bucket.example", Bucket: "wt-media", MaxBytes: 64, Inspect: func(context.Context, string, string) (string, string, media.Info, error) {
		return "image", "image/png", media.Info{Width: 640, Height: 360}, nil
	}}, r, o
}

func TestUploadRequiresNameAndNeverPublishesFailedObject(t *testing.T) {
	s, r, o := testService()
	a := operator(1, 10, identity.RoleOperator)
	for _, name := range []string{"", "../逃逸", "a/b", "bad:name"} {
		if _, err := s.Upload(context.Background(), a, name, "image.png", strings.NewReader("png")); !errors.Is(err, ErrInvalid) {
			t.Fatalf("name=%q err=%v", name, err)
		}
	}
	o.uploadErr = errors.New("bucket unavailable")
	if _, err := s.Upload(context.Background(), a, "开头图片", "image.png", strings.NewReader("png")); err == nil {
		t.Fatal("upload failure became success")
	}
	items, _, _ := r.List(model.Scope{All: true}, "", "", 1, 20)
	if len(items) != 0 {
		t.Fatal("unverified upload visible")
	}
}
func TestVerifiedFileHasIDNameStableURLAndRenameKeepsObject(t *testing.T) {
	s, r, o := testService()
	a := operator(1, 10, identity.RoleOperator)
	f, err := s.Upload(context.Background(), a, "开头图片.png", "original.png", strings.NewReader("png"))
	if err != nil {
		t.Fatal(err)
	}
	stored := r.records[f.ID]
	if stored.DeletedAt != nil {
		t.Fatalf("uploaded file deleted_at=%v", stored.DeletedAt)
	}
	if stored.ObjectKey != "s3://wt-media/file-ku/files/1_%E5%BC%80%E5%A4%B4%E5%9B%BE%E7%89%87.png" || f.Filename != "开头图片.png" || !strings.HasPrefix(f.ResourcePath, "/api/v1/file-resources/") {
		t.Fatalf("file=%+v stored=%+v", f, stored)
	}
	changed, err := s.Rename(a, f.ID, "新名称")
	if err != nil || changed.ResourcePath != f.ResourcePath || changed.PublicURL != f.PublicURL || r.records[f.ID].ObjectKey != stored.ObjectKey || len(o.contents) != 1 {
		t.Fatalf("rename=%+v err=%v", changed, err)
	}
}
func TestFileScopeAndDeleteRevokeResource(t *testing.T) {
	s, _, o := testService()
	a := operator(1, 10, identity.RoleOperator)
	f, err := s.Upload(context.Background(), a, "视频", "original.png", strings.NewReader("png"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		a    identity.PublicUser
		want bool
	}{{a, true}, {operator(2, 10, identity.RoleOperator), false}, {operator(2, 10, identity.RoleSeniorOperator), true}, {operator(3, 20, identity.RoleSeniorOperator), false}, {operator(9, 20, identity.RoleAdmin), true}} {
		_, err := s.Read(tc.a, f.ID)
		if (err == nil) != tc.want {
			t.Fatalf("actor=%+v err=%v", tc.a, err)
		}
	}
	token := strings.TrimPrefix(f.ResourcePath, "/api/v1/file-resources/")
	if _, err := s.Resource(context.Background(), token, false); err != nil {
		t.Fatal(err)
	}
	o.removeErr = errors.New("storage unavailable")
	if err := s.Delete(context.Background(), a, f.ID); err == nil {
		t.Fatal("failed delete claimed success")
	}
	if _, err := s.Resource(context.Background(), token, false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleting file still accessible: %v", err)
	}
	o.removeErr = nil
	if err := s.Delete(context.Background(), a, f.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Resource(context.Background(), token, false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted resource usable: %v", err)
	}
}

func TestVideoCannotBypassContainerValidationByRenaming(t *testing.T) {
	for _, tc := range []struct {
		ext, format, codec string
		duration           float64
		valid              bool
	}{
		{"mp4", "gif", "gif", 6, false}, {"mp4", "matroska,webm", "vp9", 6, false}, {"mp4", "mov,mp4,m4a,3gp,3g2,mj2", "h264", 6, true}, {"mp4", "mov,mp4", "h264", 0, false}, {"webm", "matroska,webm", "vp9", 6, true}, {"png", "png_pipe", "png", 0, true}, {"png", "mov,mp4", "h264", 6, false},
	} {
		_, _, err := verifiedType(media.Info{Format: tc.format, VideoCodec: tc.codec, Duration: tc.duration}, tc.ext)
		if (err == nil) != tc.valid {
			t.Fatalf("%+v err=%v", tc, err)
		}
	}
}

func TestPublicURLUsesStoredNameAfterDisplayRename(t *testing.T) {
	s, _, _ := testService()
	actor := operator(1, 10, identity.RoleOperator)
	f, err := s.Upload(context.Background(), actor, "开头 100%#1", "a.png", strings.NewReader("png"))
	if err != nil {
		t.Fatal(err)
	}
	want := "https://wt-media.bucket.example/file-ku/files/1_%E5%BC%80%E5%A4%B4%20100%25%231.png"
	if f.PublicURL != want {
		t.Fatalf("public URL: %q", f.PublicURL)
	}
	renamed, err := s.Rename(actor, f.ID, "新名")
	if err != nil || renamed.PublicURL != want {
		t.Fatalf("rename: %+v %v", renamed, err)
	}
	list, _, err := s.List(actor, "", "", 1, 20)
	if err != nil || len(list) != 1 || list[0].PublicURL != want {
		t.Fatalf("list: %+v %v", list, err)
	}
}

func TestInvalidPublicConfigurationDoesNotAllocateOrUpload(t *testing.T) {
	for _, base := range []string{"", "http://bucket.example", "https://bucket.example?signature=x"} {
		s, r, o := testService()
		s.PublicBaseURL = base
		if _, err := s.Upload(context.Background(), operator(1, 10, identity.RoleOperator), "开头", "a.png", strings.NewReader("png")); err == nil {
			t.Fatal("invalid origin accepted")
		}
		if len(r.records) != 0 || len(o.contents) != 0 {
			t.Fatal("invalid origin caused side effects")
		}
	}
	s, r, _ := testService()
	s.Bucket = "other-bucket"
	if _, err := s.Upload(context.Background(), operator(1, 10, identity.RoleOperator), "开头", "a.png", strings.NewReader("png")); err == nil || len(r.records) != 0 {
		t.Fatal("wrong bucket accepted")
	}
}

func TestDeletingFileNeverBecomesReadableAfterStorageFailure(t *testing.T) {
	s, r, o := testService()
	a := operator(1, 10, identity.RoleOperator)
	f, err := s.Upload(context.Background(), a, "封面", "a.png", strings.NewReader("png"))
	if err != nil {
		t.Fatal(err)
	}
	o.removeErr = errors.New("storage unavailable")
	if err := s.Delete(context.Background(), a, f.ID); err == nil {
		t.Fatal("failed delete claimed success")
	}
	if r.records[f.ID].DeletedAt == nil {
		t.Fatal("failed delete lost durable recovery state")
	}
	if _, err := s.Read(a, f.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleting file selectable: %v", err)
	}
	token := strings.TrimPrefix(f.ResourcePath, "/api/v1/file-resources/")
	if _, err := s.Resource(context.Background(), token, false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleting file still grants access: %v", err)
	}
	items, _, err := s.List(a, "", "", 1, 20)
	if err != nil || len(items) != 1 || items[0].DeletedAt == nil || items[0].ResourcePath != "" || items[0].PublicURL != "" {
		t.Fatalf("deletion recovery row=%+v err=%v", items, err)
	}
	o.removeErr = nil
	if err := s.Delete(context.Background(), a, f.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.records[f.ID]; ok {
		t.Fatal("retry did not finish deletion")
	}
}

func TestDeletionTimestampSurvivesFailedRetries(t *testing.T) {
	s, _, objects := testService()
	actor := operator(1, 10, identity.RoleOperator)
	file, err := s.Upload(context.Background(), actor, "封面", "a.png", strings.NewReader("png"))
	if err != nil {
		t.Fatal(err)
	}
	objects.removeErr = errors.New("storage unavailable")
	var first string
	for i := 0; i < 2; i++ {
		if err := s.Delete(context.Background(), actor, file.ID); err == nil {
			t.Fatal("failed deletion claimed success")
		}
		items, _, err := s.List(actor, "", "", 1, 20)
		if err != nil || len(items) != 1 {
			t.Fatalf("recovery listing: %v %v", items, err)
		}
		raw, _ := json.Marshal(items[0])
		var view map[string]any
		if err := json.Unmarshal(raw, &view); err != nil {
			t.Fatal(err)
		}
		timestamp, ok := view["deleted_at"].(string)
		if !ok || timestamp == "" {
			t.Fatalf("missing deletion time: %s", raw)
		}
		if _, err := time.Parse(time.RFC3339Nano, timestamp); err != nil {
			t.Fatal(err)
		}
		if _, exists := view["delete_state"]; exists {
			t.Fatal("separate deletion state still exposed")
		}
		if i == 0 {
			first = timestamp
		} else if timestamp != first {
			t.Fatal("retry changed original deletion time")
		}
	}
	objects.removeErr = nil
	if err := s.Delete(context.Background(), actor, file.ID); err != nil {
		t.Fatal(err)
	}
}

// Reject losing the actual decoded dimensions between upload and read-back.
func TestUploadReturnsInspectedImageMetadata(t *testing.T) {
	s, _, _ := testService()
	f, err := s.Upload(context.Background(), operator(1, 10, identity.RoleOperator), "封面", "cover.png", strings.NewReader("png"))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(f)
	var v struct {
		Metadata struct {
			Width  int    `json:"width"`
			Height int    `json:"height"`
			Format string `json:"format"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	if v.Metadata.Width != 640 || v.Metadata.Height != 360 || v.Metadata.Format != "png" {
		t.Fatalf("decoded image metadata lost: %s", b)
	}
}

func TestUploadRejectsUnknownImageSizeBeforeAllocation(t *testing.T) {
	s, r, o := testService()
	s.Inspect = func(context.Context, string, string) (string, string, media.Info, error) {
		return "image", "image/png", media.Info{}, nil
	}
	_, err := s.Upload(context.Background(), operator(1, 10, identity.RoleOperator), "封面", "cover.png", strings.NewReader("png"))
	if !errors.Is(err, ErrInvalid) || len(r.records) != 0 || len(o.contents) != 0 {
		t.Fatalf("unknown dimensions accepted: %v", err)
	}
}
