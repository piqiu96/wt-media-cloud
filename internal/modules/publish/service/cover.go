package service

import (
	"context"
	"encoding/json"
	"fmt"
	filemodel "github.com/wt-media/wt-media-cloud/internal/modules/files/model"
	files "github.com/wt-media/wt-media-cloud/internal/modules/files/service"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	image "github.com/wt-media/wt-media-cloud/internal/modules/image/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
)

func verifyCoverRender(file filemodel.File, content model.Content, outputID int64) (json.RawMessage, error) {
	var metadata filemodel.ImageMetadata
	if len(file.MetadataJSON) > 0 && json.Unmarshal(file.MetadataJSON, &metadata) != nil {
		return nil, ErrInvalid
	}
	if content.CoverRender == nil {
		if metadata.RenderVersion != "" && metadata.RenderVersion != "output-thumbnail.v1" {
			return nil, ErrInvalid
		}
		return nil, nil
	}
	ref := content.CoverRender
	if ref.BackgroundOutputID != 0 && ref.BackgroundOutputID != outputID {
		return nil, ErrInvalid
	}
	snapshot, err := image.VerifyArtwork(file, image.ArtworkReference{TemplateID: ref.TemplateID, PublishedRevision: ref.PublishedRevision, Title: ref.Title, BackgroundImageAssetID: ref.BackgroundImageAssetID, BackgroundOutputID: ref.BackgroundOutputID, RenderInputHash: ref.RenderInputHash})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	raw, err := json.Marshal(snapshot)
	return raw, err
}

func readCoverFile(ctx context.Context, a identity.PublicUser, id, categoryID int64) (filemodel.File, error) {
	registered, err := image.Wired().AuthorizeImageAssetFile(a, id, "cover", categoryID)
	if err != nil {
		return filemodel.File{}, err
	}
	fs, err := files.Wired()
	if err != nil {
		return filemodel.File{}, err
	}
	if registered {
		return fs.ReadSavedReference(ctx, id)
	}
	f, err := fs.Read(a, id)
	if err != nil {
		return filemodel.File{}, err
	}
	return fs.ResolveToken(ctx, f.ResourceToken)
}

// Dimensions come from decoding on upload/render, never from client claims.
func verifyCoverSpec(file filemodel.File, width, height int) error {
	var metadata filemodel.ImageMetadata
	if file.Type != "image" || file.SizeBytes <= 0 || file.SizeBytes > 5*1024*1024 || json.Unmarshal(file.MetadataJSON, &metadata) != nil || metadata.Width <= 0 || metadata.Height <= 0 || int64(metadata.Width)*int64(height) != int64(metadata.Height)*int64(width) {
		return fmt.Errorf("%w: 封面必须为%d:%d且不超过5MB", ErrInvalid, width, height)
	}
	return nil
}

func (s *Service) preparePortraitCover(ctx context.Context, a identity.PublicUser, content model.Content, categoryID, outputID int64, saved bool) (filemodel.File, json.RawMessage, error) {
	return s.prepareSecondaryCover(ctx, a, "baidu", content, categoryID, outputID, saved)
}

func secondaryCover(platform string, c model.Content) (int64, *model.CoverRender, int, int, string) {
	if platform == "bilibili" {
		return c.SpaceCoverFileID, c.SpaceCoverRender, 16, 9, "space_cover"
	}
	return c.PortraitCoverFileID, c.PortraitCoverRender, 3, 4, "portrait_cover"
}

func verifyPrimaryCover(platform string, f filemodel.File) error {
	if platform == "bilibili" {
		return verifyCoverSpec(f, 4, 3)
	}
	return verifyCoverSpec(f, 16, 9)
}

func (s *Service) prepareSecondaryCover(ctx context.Context, a identity.PublicUser, platform string, content model.Content, categoryID, outputID int64, saved bool) (filemodel.File, json.RawMessage, error) {
	id, ref, width, height, _ := secondaryCover(platform, content)
	var file filemodel.File
	var err error
	if saved && s.SavedFile != nil {
		file, err = s.SavedFile(ctx, id)
	} else {
		file, err = s.File(ctx, a, id, categoryID)
	}
	if err != nil {
		return file, nil, err
	}
	if len(file.SHA256) != 64 || previewCover(file) {
		return file, nil, ErrInvalid
	}
	if err = verifyCoverSpec(file, width, height); err != nil {
		return file, nil, err
	}
	snapshot, err := verifyCoverRender(file, model.Content{CoverRender: ref}, outputID)
	return file, snapshot, err
}

func setSecondaryCoverSnapshot(snapshot map[string]any, platform string, file filemodel.File, render json.RawMessage) {
	_, _, _, _, key := secondaryCover(platform, model.Content{})
	snapshot[key+"_render"] = render
	snapshot[key+"_sha256"] = file.SHA256
	snapshot[key+"_name"] = file.Name
	snapshot[key+"_resource_path"] = "/api/v1/file-resources/" + file.ResourceToken
}
