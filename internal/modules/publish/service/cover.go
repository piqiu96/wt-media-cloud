package service

import (
	"encoding/json"
	"fmt"
	filemodel "github.com/wt-media/wt-media-cloud/internal/modules/files/model"
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
