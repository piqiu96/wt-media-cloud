package service

import (
	"context"
	"encoding/json"
	"fmt"
	filemodel "github.com/wt-media/wt-media-cloud/internal/modules/files/model"
	files "github.com/wt-media/wt-media-cloud/internal/modules/files/service"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/image/render"
)

// Resolve business input through its owning service; callers cannot substitute a file ID.
func (s *ArtworkService) backgroundFile(a identity.PublicUser, d render.Document, in ArtworkInput) (fileID int64, assetID uint64, outputID int64, err error) {
	if in.BackgroundOutputID < 0 {
		return 0, 0, 0, ErrInvalid
	}
	switch d.BackgroundMode {
	case "solid":
		if in.BackgroundOutputID != 0 || in.BackgroundImageAssetID != 0 {
			return 0, 0, 0, ErrInvalid
		}
		return
	case "fixed":
		if in.BackgroundOutputID != 0 || in.BackgroundImageAssetID != 0 {
			return 0, 0, 0, ErrInvalid
		}
		assetID = d.ImageAssetID
	case "business":
		if (in.BackgroundOutputID != 0) == (in.BackgroundImageAssetID != 0) {
			return 0, 0, 0, fmt.Errorf("%w: 请选择一种可用底图", ErrInvalid)
		}
		if in.BackgroundOutputID != 0 {
			if s.Output == nil {
				return 0, 0, 0, ErrInvalid
			}
			output, e := s.Output(a, in.BackgroundOutputID)
			if e != nil {
				return 0, 0, 0, e
			}
			if output.Output.ID != in.BackgroundOutputID || output.Output.Thumbnail == nil || output.Output.Thumbnail.FileID <= 0 {
				return 0, 0, 0, fmt.Errorf("%w: 成片没有可用封面，请从图片库选择底图", ErrInvalid)
			}
			return output.Output.Thumbnail.FileID, 0, in.BackgroundOutputID, nil
		}
		assetID = in.BackgroundImageAssetID
	default:
		return 0, 0, 0, ErrInvalid
	}
	if assetID == 0 || s.Assets == nil {
		return 0, 0, 0, ErrInvalid
	}
	asset, e := s.Assets.GetImageAsset(a, assetID)
	if e != nil {
		return 0, 0, 0, e
	}
	if !asset.Enabled || !asset.AllowAsBackground {
		return 0, 0, 0, fmt.Errorf("%w: 底图已停用或用途不匹配", ErrInvalid)
	}
	return asset.FileID, assetID, 0, nil
}

func artworkSnapshot(f filemodel.File) (ArtworkSnapshot, string, error) {
	var meta filemodel.ImageMetadata
	var snapshot ArtworkSnapshot
	if f.ID <= 0 || f.Type != "image" || f.DeletedAt != nil || f.SizeBytes <= 0 || len(f.SHA256) != 64 || json.Unmarshal(f.MetadataJSON, &meta) != nil || meta.Preview || meta.RenderVersion != render.Version || meta.Format != "png" || meta.Width <= 0 || meta.Height <= 0 || json.Unmarshal(meta.RenderInput, &snapshot) != nil {
		return snapshot, "", ErrInvalid
	}
	if snapshot.TemplateID == 0 || snapshot.PublishedRevision == 0 || snapshot.RendererVersion != meta.RenderVersion || snapshot.Document.Width != meta.Width || snapshot.Document.Height != meta.Height || templateHash(snapshot) != meta.RenderInputHash {
		return ArtworkSnapshot{}, "", ErrInvalid
	}
	return snapshot, meta.RenderInputHash, nil
}

// A frozen, previously generated artifact remains usable after the source template changes.
func VerifyArtwork(f filemodel.File, ref ArtworkReference) (ArtworkSnapshot, error) {
	snapshot, hash, err := artworkSnapshot(f)
	if err != nil {
		return snapshot, err
	}
	if ref.RenderInputHash != hash || ref.TemplateID != snapshot.TemplateID || ref.PublishedRevision != snapshot.PublishedRevision || ref.Title != snapshot.Title || ref.BackgroundImageAssetID != snapshot.BackgroundImageAssetID || ref.BackgroundOutputID != snapshot.BackgroundOutputID {
		return ArtworkSnapshot{}, fmt.Errorf("%w: 封面配置已更改，请重新生成封面", ErrInvalid)
	}
	return snapshot, nil
}

func (s *ArtworkService) GetCover(ctx context.Context, a identity.PublicUser, id int64) (Artwork, error) {
	svc, err := s.FileService()
	if err != nil {
		return Artwork{}, err
	}
	f, err := svc.Read(a, id)
	if err != nil {
		return Artwork{}, err
	}
	f, err = svc.ResolveToken(ctx, f.ResourceToken)
	if err != nil {
		return Artwork{}, err
	}
	snapshot, hash, err := artworkSnapshot(f)
	if err != nil {
		return Artwork{}, err
	}
	return Artwork{File: ArtworkFile{FileView: filemodel.FileView{ID: f.ID, Name: f.Name, Type: f.Type, ContentType: f.ContentType, SizeBytes: f.SizeBytes, SHA256: f.SHA256, Metadata: f.MetadataJSON, ResourcePath: files.ResourcePath(f.ResourceToken)}, ID: f.ID, OwnerID: f.OwnerID, TeamID: f.TeamID}, RenderInputHash: hash, Snapshot: snapshot}, nil
}
