package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	Delete(int64) error
	List(model.Scope, string, string, int, int) ([]model.File, int64, error)
}
type Service struct {
	Repo     Repository
	Objects  storage.ImmutableStore
	MaxBytes int64
	GrantTTL time.Duration
	Inspect  func(context.Context, string, string) (string, string, error)
}

func Wired() (*Service, error) {
	cfg := config.Get()
	objects, ok := storage.Get().(storage.ImmutableStore)
	if !ok {
		return nil, storage.ErrNotConfigured
	}
	return &Service{Repo: repository.Store{DB: database.DB()}, Objects: objects, MaxBytes: cfg.Storage.ObjectStorage.UploadMaxBytes, GrantTTL: cfg.Storage.ObjectStorage.PresignTTL.Duration, Inspect: func(ctx context.Context, p, ext string) (string, string, error) {
		info, err := media.ProbeFile(ctx, cfg.Media.FFmpeg.FFprobePath, p)
		if err != nil {
			return "", "", fmt.Errorf("%w: 文件无法读取", ErrInvalid)
		}
		return verifiedType(info, ext)
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
func view(f model.File) model.View {
	return model.View{ID: f.ID, Name: f.Name, Filename: f.Name + "." + f.Extension, Type: f.Type, ContentType: f.ContentType, SizeBytes: f.SizeBytes, SHA256: f.SHA256, OwnerID: f.OwnerID, OwnerName: f.OwnerName, TeamID: f.TeamID, TeamName: f.TeamName, CreatedAt: f.CreatedAt, ResourcePath: "/api/v1/file-resources/" + f.ResourceToken}
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
	if !canRead(sc, f) {
		return model.File{}, ErrNotFound
	}
	return f, nil
}
func (s *Service) List(actor identity.PublicUser, kind, search string, page, size int) ([]model.View, int64, error) {
	sc, err := scope(actor)
	if err != nil {
		return nil, 0, err
	}
	if (kind != "" && kind != "image" && kind != "video") || page <= 0 || size <= 0 || size > 100 || utf8.RuneCountInString(search) > 100 {
		return nil, 0, ErrInvalid
	}
	items, total, err := s.Repo.List(sc, kind, strings.TrimSpace(search), page, size)
	out := make([]model.View, 0, len(items))
	for _, f := range items {
		out = append(out, view(f))
	}
	return out, total, err
}

func (s *Service) Upload(ctx context.Context, actor identity.PublicUser, name, original string, body io.Reader) (model.View, error) {
	if _, err := scope(actor); err != nil {
		return model.View{}, err
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(original)), ".")
	kind, contentType := mediaType(ext)
	if kind == "" {
		return model.View{}, ErrInvalid
	}
	name, err := filename(name, ext)
	if err != nil {
		return model.View{}, err
	}
	if s.MaxBytes <= 0 {
		return model.View{}, errors.New("file upload limit is not configured")
	}
	local, err := os.CreateTemp("", "wt-media-file-*."+ext)
	if err != nil {
		return model.View{}, err
	}
	defer os.Remove(local.Name())
	defer local.Close()
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(local, hash), io.LimitReader(body, s.MaxBytes+1))
	if err != nil {
		return model.View{}, err
	}
	if size > s.MaxBytes {
		return model.View{}, ErrTooLarge
	}
	if size <= 0 {
		return model.View{}, ErrInvalid
	}
	actualKind, actualType, err := s.Inspect(ctx, local.Name(), ext)
	if err != nil {
		return model.View{}, err
	}
	if actualKind != kind || actualType != contentType {
		return model.View{}, ErrInvalid
	}
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return model.View{}, err
	}
	now := time.Now().UTC()
	f := model.File{OwnerID: int64(actor.ID), Name: name, Extension: ext, Type: kind, ContentType: contentType, ResourceToken: hex.EncodeToString(token[:]), CreatedAt: now, UpdatedAt: now, OwnerName: actor.Nickname, TeamName: actor.TeamName}
	if f.OwnerName == "" {
		f.OwnerName = actor.Username
	}
	if actor.TeamID != nil {
		team := int64(*actor.TeamID)
		f.TeamID = &team
	}
	if err := s.Repo.Insert(&f); err != nil {
		return model.View{}, err
	}
	f.ObjectKey, err = storage.Reference(s.Objects, fmt.Sprintf("file-ku/files/%d_%s.%s", f.ID, f.Name, ext))
	if err != nil {
		return model.View{}, err
	}
	if err := s.Repo.SetLocation(f.ID, f.ObjectKey); err != nil {
		return model.View{}, err
	}
	f.SizeBytes, f.SHA256 = size, hex.EncodeToString(hash.Sum(nil))
	if _, err := local.Seek(0, io.SeekStart); err != nil {
		return model.View{}, err
	}
	// An interrupted upload stays private. Never delete on an uncertain write or
	// Ready response: the server may have committed even when the reply was lost.
	if err := s.Objects.PutImmutable(ctx, f.ObjectKey, local, size, contentType, f.SHA256); err != nil {
		return model.View{}, fmt.Errorf("upload file %d: %w", f.ID, err)
	}
	info, err := s.Objects.Stat(ctx, f.ObjectKey)
	if err != nil {
		return model.View{}, err
	}
	if info.Size != size {
		return model.View{}, errors.New("uploaded file size verification failed")
	}
	f.UploadedAt = &now
	if err := s.Repo.Ready(f); err != nil {
		return model.View{}, err
	}
	return view(f), nil
}

func (s *Service) Rename(actor identity.PublicUser, id int64, name string) (model.View, error) {
	f, err := s.Read(actor, id)
	if err != nil {
		return model.View{}, err
	}
	name, err = filename(name, f.Extension)
	if err != nil {
		return model.View{}, err
	}
	if name != f.Name {
		if err := s.Repo.Rename(id, name); err != nil {
			return model.View{}, err
		}
	}
	f.Name = name
	return view(f), nil
}
func (s *Service) Delete(ctx context.Context, actor identity.PublicUser, id int64) error {
	f, err := s.Read(actor, id)
	if err != nil {
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
