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
	"github.com/wt-media/wt-media-cloud/internal/modules/production/compose"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"strconv"
)

func OutputThumbnailWriter(binary string) func(context.Context, model.ComposeTask, int64, string, string) (int64, error) {
	return func(ctx context.Context, task model.ComposeTask, outputID int64, videoSHA256, videoPath string) (int64, error) {
		owner := task.CreatedBy
		if task.OperatorID != nil {
			owner = *task.OperatorID
		}
		actor, found, err := identity.ResolveUser(identity.UserID(owner))
		if err != nil {
			return 0, err
		}
		if !found || actor.Status != identity.UserStatusEnabled {
			return 0, ErrComposeOutputForbidden
		}
		data, err := compose.ExtractThumbnail(ctx, binary, videoPath)
		if err != nil {
			return 0, err
		}
		provenance, _ := json.Marshal(struct {
			OutputID    string `json:"composite_output_id"`
			VideoSHA256 string `json:"video_sha256"`
			FrameMS     int    `json:"frame_ms"`
		}{strconv.FormatInt(outputID, 10), videoSHA256, 0})
		digest := sha256.Sum256(provenance)
		svc, err := files.Wired()
		if err != nil {
			return 0, err
		}
		image, err := svc.UploadGeneratedImage(ctx, actor, fmt.Sprintf("成片%d缩略图", outputID), bytes.NewReader(data), filemodel.ImageMetadata{RenderInputHash: hex.EncodeToString(digest[:]), RenderVersion: "output-thumbnail.v1", RenderInput: provenance})
		return image.ID, err
	}
}

func (s *ComposeOutputService) decorateThumbnail(ctx context.Context, o *model.CompositeOutput) error {
	o.Thumbnail = nil
	id, err := o.ThumbnailFileID()
	if err != nil {
		return ErrComposeOutputVerificationFailed
	}
	if id == 0 {
		return nil
	}
	if s.thumbnailFile == nil {
		return ErrComposeOutputVerificationFailed
	}
	f, err := s.thumbnailFile(ctx, id)
	if err != nil {
		return fmt.Errorf("%w: thumbnail unavailable", ErrComposeOutputVerificationFailed)
	}
	var meta filemodel.ImageMetadata
	var origin struct {
		OutputID    string `json:"composite_output_id"`
		VideoSHA256 string `json:"video_sha256"`
		FrameMS     int    `json:"frame_ms"`
	}
	if f.ID != id || f.Type != "image" || f.DeletedAt != nil || json.Unmarshal(f.MetadataJSON, &meta) != nil || meta.RenderVersion != "output-thumbnail.v1" || meta.Preview || meta.Width <= 0 || meta.Height <= 0 || meta.Format != "png" || json.Unmarshal(meta.RenderInput, &origin) != nil || origin.OutputID != strconv.FormatInt(o.ID, 10) || origin.VideoSHA256 != o.SHA256 || origin.FrameMS != 0 {
		return ErrComposeOutputVerificationFailed
	}
	o.Thumbnail = &model.ComposeOutputThumbnail{FileID: id, ResourcePath: files.ResourcePath(f.ResourceToken), Width: meta.Width, Height: meta.Height}
	return nil
}

func RemoveOutputThumbnail(ctx context.Context, task model.ComposeTask, id int64) error {
	owner := task.CreatedBy
	if task.OperatorID != nil {
		owner = *task.OperatorID
	}
	actor, found, err := identity.ResolveUser(identity.UserID(owner))
	if err != nil {
		return err
	}
	if !found {
		return ErrComposeOutputForbidden
	}
	svc, err := files.Wired()
	if err != nil {
		return err
	}
	file, err := svc.Read(actor, id)
	if err != nil {
		return err
	}
	var meta filemodel.ImageMetadata
	if json.Unmarshal(file.MetadataJSON, &meta) != nil || meta.RenderVersion != "output-thumbnail.v1" {
		return ErrComposeOutputInvalid
	}
	return svc.Delete(ctx, actor, id)
}
