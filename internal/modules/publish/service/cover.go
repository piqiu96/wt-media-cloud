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
	var file filemodel.File
	var err error
	if saved && s.SavedFile != nil {
		file, err = s.SavedFile(ctx, content.PortraitCoverFileID)
	} else {
		file, err = s.File(ctx, a, content.PortraitCoverFileID, categoryID)
	}
	if err != nil {
		return file, nil, err
	}
	if len(file.SHA256) != 64 || previewCover(file) {
		return file, nil, ErrInvalid
	}
	if err = verifyCoverSpec(file, 3, 4); err != nil {
		return file, nil, err
	}
	ref := model.Content{CoverRender: content.PortraitCoverRender}
	snapshot, err := verifyCoverRender(file, ref, outputID)
	return file, snapshot, err
}
