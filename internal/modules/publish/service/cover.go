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
