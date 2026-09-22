package service

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/contentpool/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/contentpool/model"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
)

type (
	Status            = model.Status
	SourceContent     = model.SourceContent
	SourceContentView = model.SourceContentView
	Material          = model.Material
	SourceInput       = dto.SourceInput
	Filter            = dto.Filter
)

const (
	StatusPending         = model.StatusPending
	StatusMaterialCreated = model.StatusMaterialCreated
	StatusIgnored         = model.StatusIgnored
)

var (
	ErrInvalidInput      = errors.New("content pool input is invalid")
	ErrForbidden         = errors.New("content pool operation is forbidden")
	ErrNotFound          = errors.New("content pool item was not found")
	ErrInvalidTransition = errors.New("content pool status transition is invalid")
	ErrDuplicate         = errors.New("content pool item already exists")
)

type contentStore interface {
	CreateSource(model.SourceContent, json.RawMessage) (model.SourceContent, error)
	ListSources(dto.Filter) ([]model.SourceContentView, error)
	FindSource(int64) (model.SourceContentView, bool, error)
	UpdateStatus(int64, model.Status, string, string) (model.SourceContent, error)
	RecordMaterialFailure(int64, string) (model.SourceContent, error)
	Materialize(int64, identityservice.UserID, time.Time) (model.Material, error)
}

type contentService struct {
	store contentStore
	now   func() time.Time
}

func newContentService(store contentStore) *contentService {
	return &contentService{store: store, now: time.Now}
}

func (s *contentService) scope(actor identityservice.PublicUser, requested *identityservice.TeamID) (*identityservice.TeamID, error) {
	if actor.Role == identityservice.RoleAdmin {
		return requested, nil
	}
	if actor.TeamID == nil || (requested != nil && *requested != *actor.TeamID) {
		return nil, ErrForbidden
	}
	team := *actor.TeamID
	return &team, nil
}

func (s *contentService) createSource(actor identityservice.PublicUser, input dto.SourceInput) (model.SourceContent, error) {
	team, err := s.scope(actor, input.TeamID)
	if err != nil {
		return model.SourceContent{}, err
	}
	if team == nil || strings.TrimSpace(input.Platform) == "" || strings.TrimSpace(input.PlatformContentID) == "" || strings.TrimSpace(input.SourceType) == "" {
		return model.SourceContent{}, ErrInvalidInput
	}
	raw := input.RawJSON
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	if !json.Valid(raw) {
		return model.SourceContent{}, ErrInvalidInput
	}
	now := s.now()
	item, err := s.store.CreateSource(model.SourceContent{TeamID: *team, Platform: strings.TrimSpace(input.Platform), PlatformContentID: strings.TrimSpace(input.PlatformContentID), Title: input.Title, Description: input.Description, CoverURL: input.CoverURL, SourceURL: input.SourceURL, AuthorID: input.AuthorID, AuthorName: input.AuthorName, SourceType: input.SourceType, StrategyID: input.StrategyID, CrawlTaskID: input.CrawlTaskID, LikeCount: input.LikeCount, FavoriteCount: input.FavoriteCount, PublishedAt: input.PublishedAt, AuditNote: input.AuditNote, Status: model.StatusPending, CreatedBy: actor.ID, CreatedAt: now, UpdatedAt: now}, raw)
	if errors.Is(err, ErrDuplicate) {
		return model.SourceContent{}, ErrDuplicate
	}
	return item, err
}

func (s *contentService) list(actor identityservice.PublicUser, filter dto.Filter) ([]model.SourceContentView, error) {
	scope, err := s.scope(actor, filter.TeamID)
	if err != nil {
		return nil, err
	}
	filter.TeamID = scope
	return s.store.ListSources(filter)
}

func (s *contentService) get(actor identityservice.PublicUser, id int64) (model.SourceContentView, bool, error) {
	item, found, err := s.store.FindSource(id)
	if err != nil || !found {
		return item, found, err
	}
	if _, err := s.scope(actor, &item.TeamID); err != nil {
		return model.SourceContentView{}, false, err
	}
	return item, true, nil
}

func (s *contentService) setStatus(actor identityservice.PublicUser, id int64, status model.Status, reason string, auditNote string) (model.SourceContent, error) {
	item, found, err := s.get(actor, id)
	if err != nil {
		return model.SourceContent{}, err
	}
	if !found {
		return model.SourceContent{}, ErrNotFound
	}
	if status != model.StatusIgnored && status != model.StatusPending {
		return model.SourceContent{}, ErrInvalidInput
	}
	if item.Status == model.StatusMaterialCreated && status == model.StatusPending {
		return model.SourceContent{}, ErrInvalidTransition
	}
	return s.store.UpdateStatus(id, status, reason, auditNote)
}

func (s *contentService) batchSetStatus(actor identityservice.PublicUser, ids []int64, status model.Status, reason string, auditNote string) (dto.BatchOperationResponse, error) {
	if len(ids) == 0 || len(ids) > 500 {
		return dto.BatchOperationResponse{}, ErrInvalidInput
	}
	result := dto.BatchOperationResponse{Items: make([]dto.BatchOperationItem, 0, len(ids))}
	for _, id := range ids {
		item, err := s.setStatus(actor, id, status, reason, auditNote)
		if err != nil {
			result.Failed++
			result.Items = append(result.Items, dto.BatchOperationItem{ID: id, Message: err.Error()})
			continue
		}
		result.Succeeded++
		result.Items = append(result.Items, dto.BatchOperationItem{ID: id, Success: true, Source: &item})
	}
	return result, nil
}

func (s *contentService) batchMaterialize(actor identityservice.PublicUser, ids []int64) dto.BatchOperationResponse {
	result := dto.BatchOperationResponse{Items: make([]dto.BatchOperationItem, 0, len(ids))}
	for _, id := range ids {
		material, err := s.materialize(actor, id)
		if err != nil {
			result.Failed++
			result.Items = append(result.Items, dto.BatchOperationItem{ID: id, Message: err.Error()})
			continue
		}
		source, found, err := s.get(actor, id)
		if err != nil || !found {
			result.Failed++
			result.Items = append(result.Items, dto.BatchOperationItem{ID: id, Message: ErrNotFound.Error()})
			continue
		}
		result.Succeeded++
		result.Items = append(result.Items, dto.BatchOperationItem{ID: id, Success: true, Source: &source.SourceContent, Material: &material})
	}
	return result
}

func (s *contentService) materialize(actor identityservice.PublicUser, id int64) (model.Material, error) {
	_, found, err := s.get(actor, id)
	if err != nil {
		return model.Material{}, err
	}
	if !found {
		return model.Material{}, ErrNotFound
	}
	material, err := s.store.Materialize(id, actor.ID, s.now())
	if err != nil {
		_, _ = s.store.RecordMaterialFailure(id, err.Error())
		return model.Material{}, err
	}
	return material, nil
}
