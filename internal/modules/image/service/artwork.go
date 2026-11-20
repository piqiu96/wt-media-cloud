package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	filemodel "github.com/wt-media/wt-media-cloud/internal/modules/files/model"
	files "github.com/wt-media/wt-media-cloud/internal/modules/files/service"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/image/render"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

var artworkSlots = make(chan struct{}, 4)

type ArtworkService struct {
	Assets      *Service
	FileService func() (*files.Service, error)
}

func WiredArtwork() *ArtworkService {
	return &ArtworkService{Assets: Wired(), FileService: files.Wired}
}

type ArtworkInput struct {
	TemplateID             uint64 `json:"template_id,string"`
	PublishedRevision      uint32 `json:"expected_published_revision"`
	Title                  string `json:"title"`
	BackgroundImageAssetID uint64 `json:"background_image_asset_id,string"`
	RowKey                 string `json:"row_key"`
	InputRevision          string `json:"input_revision"`
}
type ArtworkSnapshot struct {
	TemplateID             uint64          `json:"template_id,string"`
	PublishedRevision      uint32          `json:"published_revision"`
	Document               render.Document `json:"document"`
	Title                  string          `json:"title"`
	BackgroundImageAssetID uint64          `json:"background_image_asset_id,string"`
	BackgroundFileID       int64           `json:"background_file_id,string"`
	RendererVersion        string          `json:"renderer_version"`
}
type ArtworkFile struct {
	filemodel.FileView
	ID      int64  `json:"id,string"`
	OwnerID int64  `json:"owner_id,string"`
	TeamID  *int64 `json:"team_id,string"`
}

type Artwork struct {
	File            ArtworkFile     `json:"file"`
	RenderInputHash string          `json:"render_input_hash"`
	Snapshot        ArtworkSnapshot `json:"snapshot"`
	Preview         bool            `json:"preview"`
	RowKey          string          `json:"row_key"`
	InputRevision   string          `json:"input_revision"`
}

func (s *ArtworkService) RenderDocument(ctx context.Context, a identity.PublicUser, d render.Document, title string, bgID uint64, preview bool, input ArtworkInput) (Artwork, []render.Issue, error) {
	result := Artwork{Preview: preview, RowKey: input.RowKey, InputRevision: input.InputRevision}
	select {
	case artworkSlots <- struct{}{}:
		defer func() { <-artworkSlots }()
	case <-ctx.Done():
		return result, nil, ctx.Err()
	}
	if !utf8.ValidString(title) || strings.TrimSpace(title) == "" || utf8.RuneCountInString(title) > 2000 {
		return result, nil, ErrInvalid
	}
	fileService, e := s.FileService()
	if e != nil {
		return result, nil, e
	}
	var background image.Image
	var backgroundFileID int64
	if d.BackgroundMode != "solid" {
		id := bgID
		if d.BackgroundMode == "fixed" {
			id = d.ImageAssetID
		}
		if id == 0 {
			return result, nil, fmt.Errorf("%w: 请选择可用底图", ErrInvalid)
		}
		asset, e := s.Assets.GetImageAsset(a, id)
		if e != nil {
			return result, nil, e
		}
		if !asset.Enabled || !asset.AllowAsBackground {
			return result, nil, fmt.Errorf("%w: 底图已停用或用途不匹配", ErrInvalid)
		}
		f, e := fileService.Read(a, asset.FileID)
		if e != nil {
			return result, nil, e
		}
		if f.DeletedAt != nil || f.SizeBytes <= 0 || f.SizeBytes > fileService.MaxBytes {
			return result, nil, ErrInvalid
		}
		reader, e := fileService.Objects.Open(ctx, f.ObjectKey)
		if e != nil {
			return result, nil, e
		}
		data, readErr := io.ReadAll(io.LimitReader(reader, fileService.MaxBytes+1))
		reader.Close()
		if readErr != nil {
			return result, nil, readErr
		}
		if int64(len(data)) != f.SizeBytes {
			return result, nil, fmt.Errorf("底图文件不完整")
		}
		digest := sha256.Sum256(data)
		if hex.EncodeToString(digest[:]) != f.SHA256 {
			return result, nil, fmt.Errorf("底图校验失败，请重新上传")
		}
		info, _, e := image.DecodeConfig(bytes.NewReader(data))
		if e != nil || info.Width <= 0 || info.Height <= 0 || int64(info.Width)*int64(info.Height) > 32000000 {
			return result, nil, fmt.Errorf("底图无法读取或尺寸过大")
		}
		background, _, e = image.Decode(bytes.NewReader(data))
		if e != nil {
			return result, nil, e
		}
		backgroundFileID = f.ID
		bgID = id
	} else {
		bgID = 0
	}
	result.Snapshot = ArtworkSnapshot{TemplateID: input.TemplateID, PublishedRevision: input.PublishedRevision, Document: d, Title: title, BackgroundImageAssetID: bgID, BackgroundFileID: backgroundFileID, RendererVersion: render.Version}
	// Resource paths are presentation data, not render input identity.
	result.Snapshot.Document.Image = ""
	result.RenderInputHash = templateHash(result.Snapshot)
	data, issues, e := render.Render(ctx, d, title, background)
	if e != nil {
		return result, issues, fmt.Errorf("%w: %s", ErrInvalid, e)
	}
	provenance, _ := json.Marshal(result.Snapshot)
	prefix := "封面"
	if preview {
		prefix = "预览样例"
	}
	stored, e := fileService.UploadGeneratedImage(ctx, a, prefix+"-"+result.RenderInputHash[:16], bytes.NewReader(data), filemodel.ImageMetadata{RenderInputHash: result.RenderInputHash, RenderVersion: render.Version, RenderInput: provenance, Preview: preview})
	result.File = ArtworkFile{FileView: stored, ID: stored.ID, OwnerID: stored.OwnerID, TeamID: stored.TeamID}
	return result, issues, e
}
func (s *TemplateService) RenderCover(ctx context.Context, a identity.PublicUser, in ArtworkInput) (Artwork, []render.Issue, error) {
	v, e := s.GetCoverTemplate(a, in.TemplateID)
	if e != nil {
		return Artwork{}, nil, e
	}
	if !v.Enabled || v.PublishedRevision == 0 || v.PublishedRevision != in.PublishedRevision {
		return Artwork{}, nil, ErrConflict
	}
	if in.RowKey == "" || len(in.RowKey) > 100 || in.InputRevision == "" || len(in.InputRevision) > 100 {
		return Artwork{}, nil, ErrInvalid
	}
	var d render.Document
	if json.Unmarshal(v.PublishedConfig, &d) != nil {
		return Artwork{}, nil, ErrInvalid
	}
	return s.Artwork.RenderDocument(ctx, a, d, in.Title, in.BackgroundImageAssetID, false, in)
}
func (s *TemplateService) ValidateCoverTemplate(ctx context.Context, a identity.PublicUser, id uint64, in ValidateTemplateInput) (Artwork, []render.Issue, error) {
	v, e := s.GetCoverTemplate(a, id)
	if e != nil {
		return Artwork{}, nil, e
	}
	if v.Revision != in.ExpectedRevision {
		return Artwork{}, nil, ErrConflict
	}
	var d render.Document
	if in.Document != nil {
		d = *in.Document
	} else {
		data := v.DraftConfig
		if len(data) == 0 {
			data = v.PublishedConfig
		}
		if json.Unmarshal(data, &d) != nil {
			return Artwork{}, nil, ErrInvalid
		}
	}
	if d.Width != v.CanvasWidth || d.Height != v.CanvasHeight {
		return Artwork{}, nil, ErrInvalid
	}
	return s.Artwork.RenderDocument(ctx, a, d, in.Title, in.BackgroundImageAssetID, true, ArtworkInput{TemplateID: id, PublishedRevision: v.PublishedRevision, RowKey: strconv.FormatUint(id, 10), InputRevision: strconv.FormatUint(in.ExpectedRevision, 10)})
}

type ValidateTemplateInput struct {
	ExpectedRevision       uint64           `json:"expected_revision,string"`
	Document               *render.Document `json:"document"`
	Title                  string           `json:"title"`
	BackgroundImageAssetID uint64           `json:"background_image_asset_id,string"`
}
