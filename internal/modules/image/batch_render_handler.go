package image

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cloudwego/hertz/pkg/app"
	files "github.com/wt-media/wt-media-cloud/internal/modules/files/service"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/image/service"
	"github.com/wt-media/wt-media-cloud/internal/shared/api"
)

type batchCoverSpec struct {
	Specification          string `json:"specification"`
	BackgroundOutputID     int64  `json:"background_output_id,string"`
	TemplateID             uint64 `json:"template_id,string"`
	PublishedRevision      uint32 `json:"expected_published_revision"`
	Title                  string `json:"title"`
	BackgroundImageAssetID uint64 `json:"background_image_asset_id,string"`
}

type batchCoverRow struct {
	RowKey         string            `json:"row_key"`
	InputRevision  string            `json:"input_revision"`
	FieldRevisions map[string]string `json:"field_revisions"`
	Platform       string            `json:"platform"`
	CategoryID     int64             `json:"category_id,string"`
	Specs          []batchCoverSpec  `json:"specs"`
}

type specifiedArtwork struct {
	service.Artwork
	Specification string `json:"specification"`
}

func isBatchCoverRequest(c *app.RequestContext) bool {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(c.Request.Body(), &fields); err != nil {
		return false
	}
	_, found := fields["items"]
	return found
}

func validCoverRevision(raw string, allowZero bool) bool {
	id, err := strconv.ParseUint(raw, 10, 64)
	return err == nil && (allowZero || id > 0) && strconv.FormatUint(id, 10) == raw
}

func validBatchCoverRow(row batchCoverRow) bool {
	if row.Platform != "baidu" || row.CategoryID <= 0 ||
		row.RowKey == "" || len(row.RowKey) > 64 || strings.TrimSpace(row.RowKey) != row.RowKey ||
		!validCoverRevision(row.InputRevision, false) || len(row.FieldRevisions) == 0 || len(row.FieldRevisions) > 16 ||
		len(row.Specs) == 0 || len(row.Specs) > 2 {
		return false
	}
	for field, revision := range row.FieldRevisions {
		if field == "" || len(field) > 64 || strings.TrimSpace(field) != field || !validCoverRevision(revision, true) {
			return false
		}
	}
	seen := map[string]bool{}
	for _, spec := range row.Specs {
		if spec.Specification != "landscape" && spec.Specification != "portrait" ||
			seen[spec.Specification] || spec.TemplateID == 0 || spec.PublishedRevision == 0 ||
			strings.TrimSpace(spec.Title) == "" || utf8.RuneCountInString(spec.Title) > 2000 ||
			spec.BackgroundOutputID < 0 || spec.BackgroundOutputID > 0 && spec.BackgroundImageAssetID > 0 {
			return false
		}
		seen[spec.Specification] = true
	}
	return true
}

func renderBatchCover(c *app.RequestContext, a identity.PublicUser) {
	if a.ID <= 0 || a.Status != identity.UserStatusEnabled || a.Role != identity.RoleOperator && a.Role != identity.RoleSeniorOperator {
		failTemplate(c, service.ErrNotFound)
		return
	}
	var in struct {
		Items []batchCoverRow `json:"items"`
	}
	if decodeInput(c, &in) != nil || len(in.Items) == 0 || len(in.Items) > api.MaxStreamItems {
		failTemplate(c, service.ErrInvalid)
		return
	}
	seen := make(map[string]bool, len(in.Items))
	jobs := make([]api.StreamJob, 0, len(in.Items))
	for _, row := range in.Items {
		if seen[row.RowKey] || !validBatchCoverRow(row) {
			failTemplate(c, service.ErrInvalid)
			return
		}
		if a.TeamID == nil || !a.CanAccess(a.ID, a.TeamID, identity.CategoryID(row.CategoryID)) {
			failTemplate(c, service.ErrNotFound)
			return
		}
		seen[row.RowKey] = true
		jobs = append(jobs, api.StreamJob{
			Item: api.StreamItem{RowKey: row.RowKey, InputRevision: row.InputRevision, FieldRevisions: row.FieldRevisions}, Stage: "rendering",
			Run: func(ctx context.Context) (any, error) {
				templates := service.WiredTemplate()
				out := make([]specifiedArtwork, 0, len(row.Specs))
				for _, spec := range row.Specs {
					if err := ctx.Err(); err != nil {
						return nil, err
					}
					width, height := 16, 9
					if spec.Specification == "portrait" {
						width, height = 3, 4
					}
					artwork, issues, err := templates.RenderCover(ctx, a, service.ArtworkInput{
						RowKey: row.RowKey, InputRevision: row.InputRevision, TemplateID: spec.TemplateID, PublishedRevision: spec.PublishedRevision, Title: spec.Title,
						BackgroundImageAssetID: spec.BackgroundImageAssetID, BackgroundOutputID: spec.BackgroundOutputID,
						CategoryID: row.CategoryID, Platform: row.Platform, SpecWidth: width, SpecHeight: height,
					})
					if err != nil {
						return map[string]any{"specification": spec.Specification, "issues": issues}, generationError(err)
					}
					out = append(out, specifiedArtwork{Artwork: artwork, Specification: spec.Specification})
				}
				return struct {
					Artworks       []specifiedArtwork `json:"artworks"`
					FieldRevisions map[string]string  `json:"field_revisions"`
				}{out, row.FieldRevisions}, nil
			},
		})
	}
	if _, err := files.Wired(); err != nil {
		failTemplate(c, err)
		return
	}
	api.StreamManyWithTimeout(c, int64(a.ID), 180*time.Second, jobs)
}
