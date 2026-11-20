package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/infra/media"
	"github.com/wt-media/wt-media-cloud/internal/infra/storage"
	"github.com/wt-media/wt-media-cloud/internal/modules/files/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/files/repository"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
)

var (
	ErrInUse     = repository.ErrInUse
	ErrInvalid   = errors.New("file input invalid")
	ErrTooLarge  = errors.New("file too large")
	ErrNotFound  = repository.ErrNotFound
	ErrForbidden = errors.New("file access forbidden")
)

type Repository interface {
	Insert(*model.File) error
	Ready(model.File) error
	SetLocation(int64, string) error
	Find(int64) (model.File, error)
	FindToken(string) (model.File, error)
	Rename(int64, string) error
	BeginDelete(int64) error
	Delete(int64) error
	List(model.Scope, string, string, int, int) ([]model.File, int64, error)
}
type Service struct {
	PublicBaseURL string
	Bucket        string
	Repo          Repository
	Objects       storage.ImmutableStore
	MaxBytes      int64
	GrantTTL      time.Duration
	Inspect       func(context.Context, string, string) (string, string, media.Info, error)
}

// LockAvailable serializes saved references with deletion. Lock IDs in a stable order.
func LockAvailable(tx *gorm.DB, ids []int64) error {
	ordered := append([]int64(nil), ids...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	return repository.LockAvailable(tx, ordered)
}
func Wired() (*Service, error) {
	cfg := config.Get()
	objects, ok := storage.Get().(storage.ImmutableStore)
	if !ok {
		return nil, storage.ErrNotConfigured
	}
	return &Service{Repo: repository.Store{DB: database.DB(), CheckReferences: referenceCheck}, Objects: objects, PublicBaseURL: cfg.Storage.ObjectStorage.FilesPublicBaseURL, Bucket: cfg.Storage.ObjectStorage.Bucket, MaxBytes: cfg.Storage.ObjectStorage.UploadMaxBytes, GrantTTL: cfg.Storage.ObjectStorage.PresignTTL.Duration, Inspect: func(ctx context.Context, p, ext string) (string, string, media.Info, error) {
		info, err := media.ProbeFile(ctx, cfg.Media.FFmpeg.FFprobePath, p)
		if err != nil {
			return "", "", media.Info{}, fmt.Errorf("%w: 文件无法读取", ErrInvalid)
		}
		kind, contentType, err := verifiedType(info, ext)
		return kind, contentType, info, err
	}}, nil
}

func verifiedType(info media.Info, ext string) (string, string, error) {
	kind, contentType := mediaType(ext)
	valid := false
	if kind == "image" {
		codec := ext
		if ext == "jpg" || ext == "jpeg" {
			codec = "mjpeg"
		}
		valid = info.VideoCodec == codec
	}
	if kind == "video" && info.Duration > 0 {
		for _, format := range strings.Split(info.Format, ",") {
			switch ext {
			case "mp4", "m4v", "mov":
				valid = valid || format == "mov" || format == "mp4"
			case "mkv", "webm":
				valid = valid || format == "matroska" || format == "webm"
			}
		}
	}
	if !valid {
		return "", "", fmt.Errorf("%w: 文件内容与扩展名不一致", ErrInvalid)
	}
	return kind, contentType, nil
}

func mediaType(ext string) (string, string) {
	switch ext {
	case "png":
		return "image", "image/png"
	case "jpg", "jpeg":
		return "image", "image/jpeg"
	case "webp":
		return "image", "image/webp"
	case "gif":
		return "image", "image/gif"
	case "mp4", "m4v":
		return "video", "video/mp4"
	case "mov":
		return "video", "video/quicktime"
	case "webm":
		return "video", "video/webm"
	case "mkv":
		return "video", "video/x-matroska"
	}
	return "", ""
}

func filename(name, ext string) (string, error) {
	name = strings.TrimSpace(name)
	if strings.HasSuffix(strings.ToLower(name), "."+ext) {
		name = strings.TrimSpace(name[:len(name)-len(ext)-1])
	}
	if !utf8.ValidString(name) || name == "" || name == "." || strings.Contains(name, "..") || utf8.RuneCountInString(name) > 80 || len(name) > 160 {
		return "", ErrInvalid
	}
	for _, r := range name {
		if unicode.IsControl(r) || strings.ContainsRune(`<>:"/\|?*`, r) {
			return "", ErrInvalid
		}
	}
	if strings.HasSuffix(name, ".") {
		return "", ErrInvalid
	}
	return name, nil
}
func scope(actor identity.PublicUser) (model.Scope, error) {
	if actor.ID <= 0 || actor.Status != identity.UserStatusEnabled {
		return model.Scope{}, ErrForbidden
	}
	switch actor.Role {
	case identity.RoleAdmin:
		return model.Scope{All: true}, nil
	case identity.RoleOperator:
		return model.Scope{OwnerID: int64(actor.ID)}, nil
	case identity.RoleSeniorOperator:
		if actor.TeamID != nil && *actor.TeamID > 0 {
			return model.Scope{TeamID: int64(*actor.TeamID)}, nil
		}
	}
	return model.Scope{}, ErrForbidden
}
func canRead(sc model.Scope, f model.File) bool {
	return sc.All || (sc.OwnerID > 0 && sc.OwnerID == f.OwnerID) || (sc.TeamID > 0 && f.TeamID != nil && sc.TeamID == *f.TeamID)
}
func (s *Service) view(f model.File) (model.FileView, error) {
	if f.DeletedAt != nil {
		return model.FileView{DeletedAt: f.DeletedAt, ID: f.ID, Name: f.Name, Filename: f.Name + "." + f.Extension, Type: f.Type, SizeBytes: f.SizeBytes, OwnerID: f.OwnerID, OwnerName: f.OwnerName, TeamID: f.TeamID, TeamName: f.TeamName, CreatedAt: f.CreatedAt}, nil
	}
	publicURL, err := storage.PublicObjectURL(s.PublicBaseURL, s.Bucket, f.ObjectKey)
	if err != nil {
		return model.FileView{}, err
	}
	return model.FileView{Metadata: f.MetadataJSON, PublicURL: publicURL, ID: f.ID, Name: f.Name, Filename: f.Name + "." + f.Extension, Type: f.Type, ContentType: f.ContentType, SizeBytes: f.SizeBytes, SHA256: f.SHA256, OwnerID: f.OwnerID, OwnerName: f.OwnerName, TeamID: f.TeamID, TeamName: f.TeamName, CreatedAt: f.CreatedAt, ResourcePath: ResourcePath(f.ResourceToken)}, nil
}
func (s *Service) Read(actor identity.PublicUser, id int64) (model.File, error) {
	sc, err := scope(actor)
	if err != nil {
		return model.File{}, err
	}
	f, err := s.Repo.Find(id)
	if err != nil {
		return f, err
	}
	if !canRead(sc, f) || f.DeletedAt != nil {
		return model.File{}, ErrNotFound
	}
	return f, nil
}

// ReadSavedReference is for a business service that has already authorized a saved
// reference. It does not grant general file-list access to another owner's uploads.
func (s *Service) ReadSavedReference(ctx context.Context, id int64) (model.File, error) {
	f, err := s.SavedReference(id)
	if err != nil {
		return f, err
	}
	return s.ResolveToken(ctx, f.ResourceToken)
}
func (s *Service) List(actor identity.PublicUser, kind, search string, page, size int) ([]model.FileView, int64, error) {
	sc, err := scope(actor)
	if err != nil {
		return nil, 0, err
	}
	if (kind != "" && kind != "image" && kind != "video") || page <= 0 || size <= 0 || size > 100 || utf8.RuneCountInString(search) > 100 {
		return nil, 0, ErrInvalid
	}
	items, total, err := s.Repo.List(sc, kind, strings.TrimSpace(search), page, size)
	out := make([]model.FileView, 0, len(items))
	for _, f := range items {
		item, viewErr := s.view(f)
		if viewErr != nil {
			return nil, 0, viewErr
		}
		out = append(out, item)
	}
	return out, total, err
}

func (s *Service) Upload(ctx context.Context, actor identity.PublicUser, name, original string, body io.Reader) (model.FileView, error) {
	return s.upload(ctx, actor, name, original, body, nil)
}

// UploadGeneratedImage stores provider-owned artwork provenance with inspected dimensions.
// This is an in-process capability, never exposed as user-supplied upload metadata.
func (s *Service) UploadGeneratedImage(ctx context.Context, actor identity.PublicUser, name string, body io.Reader, provenance model.ImageMetadata) (model.FileView, error) {
	if provenance.RenderInputHash == "" || provenance.RenderVersion == "" || !json.Valid(provenance.RenderInput) {
		return model.FileView{}, ErrInvalid
	}
	return s.upload(ctx, actor, name, name+".png", body, &provenance)
}
func (s *Service) upload(ctx context.Context, actor identity.PublicUser, name, original string, body io.Reader, provenance *model.ImageMetadata) (model.FileView, error) {
	if _, err := scope(actor); err != nil {
		return model.FileView{}, err
	}
	// Validate the public origin and adapter binding before any upload allocation.
	probe, err := storage.Reference(s.Objects, "file-ku/files/_configuration-check")
	if err != nil {
		return model.FileView{}, err
	}
	if _, err = storage.PublicObjectURL(s.PublicBaseURL, s.Bucket, probe); err != nil {
		return model.FileView{}, err
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(original)), ".")
	kind, contentType := mediaType(ext)
	if kind == "" {
		return model.FileView{}, ErrInvalid
	}
	name, err = filename(name, ext)
	if err != nil {
		return model.FileView{}, err
	}
	if s.MaxBytes <= 0 {
		return model.FileView{}, errors.New("file upload limit is not configured")
	}
	local, err := os.CreateTemp("", "wt-media-file-*."+ext)
	if err != nil {
		return model.FileView{}, err
	}
	defer os.Remove(local.Name())
	defer local.Close()
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(local, hash), io.LimitReader(body, s.MaxBytes+1))
	if err != nil {
		return model.FileView{}, err
	}
	if size > s.MaxBytes {
		return model.FileView{}, ErrTooLarge
	}
	if size <= 0 {
		return model.FileView{}, ErrInvalid
	}
	actualKind, actualType, inspected, err := s.Inspect(ctx, local.Name(), ext)
	if err != nil {
		return model.FileView{}, err
	}
	if actualKind != kind || actualType != contentType {
		return model.FileView{}, ErrInvalid
	}
	var metadata json.RawMessage
	if kind == "image" {
		if inspected.Width <= 0 || inspected.Height <= 0 {
			return model.FileView{}, ErrInvalid
		}
		imageMetadata := model.ImageMetadata{Width: inspected.Width, Height: inspected.Height, Format: ext, Source: "upload"}
		if provenance != nil {
			imageMetadata.RenderInputHash = provenance.RenderInputHash
			imageMetadata.RenderVersion = provenance.RenderVersion
			imageMetadata.RenderInput = provenance.RenderInput
			imageMetadata.Preview = provenance.Preview
			imageMetadata.Source = "render"
		}
		metadata, err = json.Marshal(imageMetadata)
		if err != nil {
			return model.FileView{}, err
		}
	}
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return model.FileView{}, err
	}
	now := time.Now().UTC()
	f := model.File{MetadataJSON: metadata, OwnerID: int64(actor.ID), Name: name, Extension: ext, Type: kind, ContentType: contentType, ResourceToken: hex.EncodeToString(token[:]), CreatedAt: now, UpdatedAt: now, OwnerName: actor.Nickname, TeamName: actor.TeamName}
	if f.OwnerName == "" {
		f.OwnerName = actor.Username
	}
	if actor.TeamID != nil {
		team := int64(*actor.TeamID)
		f.TeamID = &team
	}
	if err := s.Repo.Insert(&f); err != nil {
		return model.FileView{}, err
	}
	f.ObjectKey, err = storage.Reference(s.Objects, fmt.Sprintf("file-ku/files/%d_%s.%s", f.ID, f.Name, ext))
	if err != nil {
		return model.FileView{}, err
	}
	if err := s.Repo.SetLocation(f.ID, f.ObjectKey); err != nil {
		return model.FileView{}, err
	}
	f.SizeBytes, f.SHA256 = size, hex.EncodeToString(hash.Sum(nil))
	result, err := s.view(f)
	if err != nil {
		return model.FileView{}, err
	}
	if _, err := local.Seek(0, io.SeekStart); err != nil {
		return model.FileView{}, err
	}
	// An interrupted upload is never listed as ready. Never delete on an uncertain write or
	// Ready response: the server may have committed even when the reply was lost.
	if err := s.Objects.PutImmutable(ctx, f.ObjectKey, local, size, contentType, f.SHA256); err != nil {
		return model.FileView{}, fmt.Errorf("upload file %d: %w", f.ID, err)
	}
	info, err := s.Objects.Stat(ctx, f.ObjectKey)
	if err != nil {
		return model.FileView{}, err
	}
	if info.Size != size {
		return model.FileView{}, errors.New("uploaded file size verification failed")
	}
	f.UploadedAt = &now
	if err := s.Repo.Ready(f); err != nil {
		return model.FileView{}, err
	}
	return result, nil
}

func (s *Service) Rename(actor identity.PublicUser, id int64, name string) (model.FileView, error) {
	f, err := s.Read(actor, id)
	if err != nil {
		return model.FileView{}, err
	}
	name, err = filename(name, f.Extension)
	if err != nil {
		return model.FileView{}, err
	}
	updated := f
	updated.Name = name
	result, err := s.view(updated)
	if err != nil {
		return model.FileView{}, err
	}
	if name != f.Name {
		if err := s.Repo.Rename(id, name); err != nil {
			return model.FileView{}, err
		}
	}
	return result, nil
}
func (s *Service) Delete(ctx context.Context, actor identity.PublicUser, id int64) error {
	sc, err := scope(actor)
	if err != nil {
		return err
	}
	f, err := s.Repo.Find(id)
	if err != nil {
		return err
	}
	if !canRead(sc, f) {
		return ErrNotFound
	}
	if err := s.Repo.BeginDelete(id); err != nil {
		return err
	}
	if err := s.Objects.Remove(ctx, f.ObjectKey); err != nil && !errors.Is(err, storage.ErrObjectNotFound) {
		return err
	}
	return s.Repo.Delete(id)
}

// ResolveToken is also the in-process resource provider used by composition:
// an owned file never needs a network trip back through Cloud's public address.
func (s *Service) ResolveToken(ctx context.Context, token string) (model.File, error) {
	if len(token) != 64 {
		return model.File{}, ErrNotFound
	}
	if _, err := hex.DecodeString(token); err != nil {
		return model.File{}, ErrNotFound
	}
	f, err := s.Repo.FindToken(token)
	if err == nil && f.DeletedAt != nil {
		return f, ErrNotFound
	}
	if err != nil {
		return f, err
	}
	info, err := s.Objects.Stat(ctx, f.ObjectKey)
	if errors.Is(err, storage.ErrObjectNotFound) {
		return model.File{}, ErrNotFound
	}
	if err != nil {
		return model.File{}, err
	}
	if info.Size != f.SizeBytes {
		return model.File{}, errors.New("file size verification failed")
	}
	return f, nil
}
func (s *Service) Resource(ctx context.Context, token string, download bool) (storage.Grant, error) {
	f, err := s.ResolveToken(ctx, token)
	if err != nil {
		return storage.Grant{}, err
	}
	if download {
		return storage.PresignDownload(ctx, s.Objects, f.ObjectKey, f.Name+"."+f.Extension, s.GrantTTL)
	}
	return s.Objects.PresignGet(ctx, f.ObjectKey, s.GrantTTL)
}

// SavedReference reads an authorized saved reference without signing or object I/O.
func (s *Service) SavedReference(id int64) (model.File, error) {
	f, err := s.Repo.Find(id)
	if err == nil && f.DeletedAt != nil {
		return model.File{}, ErrNotFound
	}
	return f, err
}
func ReadSavedReference(db *gorm.DB, id int64) (model.File, error) {
	return (&Service{Repo: repository.Store{DB: db}}).SavedReference(id)
}

// ResourcePath returns the file provider-owned stable access path.
func ResourcePath(token string) string { return "/api/v1/file-resources/" + token }
