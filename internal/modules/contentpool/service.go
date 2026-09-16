package contentpool

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
)

type Status string

const (
	StatusPending         Status = "pending"
	StatusMaterialCreated Status = "material_created"
	StatusIgnored         Status = "ignored"
)

type SourceContent struct {
	ID                int64           `json:"id"`
	TeamID            identity.TeamID `json:"team_id"`
	Platform          string          `json:"platform"`
	PlatformContentID string          `json:"platform_content_id"`
	Title             string          `json:"title"`
	Description       string          `json:"description,omitempty"`
	CoverURL          string          `json:"cover_url,omitempty"`
	SourceURL         string          `json:"source_url,omitempty"`
	AuthorID          string          `json:"author_id,omitempty"`
	AuthorName        string          `json:"author_name,omitempty"`
	SourceType        string          `json:"source_type"`
	PublishedAt       *time.Time      `json:"published_at,omitempty"`
	Status            Status          `json:"status"`
	IgnoredReason     string          `json:"ignored_reason,omitempty"`
	CreatedBy         identity.UserID `json:"created_by"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

type Material struct {
	ID              int64           `json:"id"`
	TeamID          identity.TeamID `json:"team_id"`
	SourceContentID int64           `json:"source_content_id"`
	Title           string          `json:"title"`
	SourceSnapshot  map[string]any  `json:"source_snapshot"`
	CreatedBy       identity.UserID `json:"created_by"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type SourceInput struct {
	TeamID            *identity.TeamID
	Platform          string
	PlatformContentID string
	Title             string
	Description       string
	CoverURL          string
	SourceURL         string
	AuthorID          string
	AuthorName        string
	SourceType        string
	PublishedAt       *time.Time
	RawJSON           json.RawMessage
}

type Filter struct {
	TeamID     *identity.TeamID
	Platform   string
	Status     Status
	SourceType string
	Search     string
}

type Store interface {
	CreateSource(SourceContent, json.RawMessage) (SourceContent, error)
	ListSources(Filter) ([]SourceContent, error)
	FindSource(int64) (SourceContent, bool, error)
	UpdateStatus(int64, Status, string) (SourceContent, error)
	Materialize(int64, identity.UserID, time.Time) (Material, error)
}

type Service struct {
	store Store
	now   func() time.Time
}

var (
	ErrInvalidInput      = errors.New("content pool input is invalid")
	ErrForbidden         = errors.New("content pool operation is forbidden")
	ErrNotFound          = errors.New("content pool item was not found")
	ErrInvalidTransition = errors.New("content pool status transition is invalid")
	ErrDuplicate         = errors.New("content pool item already exists")
)

func NewService(store Store) *Service { return &Service{store: store, now: time.Now} }

func (s *Service) Scope(actor identity.PublicUser, requested *identity.TeamID) (*identity.TeamID, error) {
	if actor.Role == identity.RoleAdmin {
		return requested, nil
	}
	if actor.TeamID == nil || (requested != nil && *requested != *actor.TeamID) {
		return nil, ErrForbidden
	}
	team := *actor.TeamID
	return &team, nil
}

func (s *Service) CreateSource(actor identity.PublicUser, input SourceInput) (SourceContent, error) {
	team, err := s.Scope(actor, input.TeamID)
	if err != nil {
		return SourceContent{}, err
	}
	if team == nil || strings.TrimSpace(input.Platform) == "" || strings.TrimSpace(input.PlatformContentID) == "" || strings.TrimSpace(input.SourceType) == "" {
		return SourceContent{}, ErrInvalidInput
	}
	raw := input.RawJSON
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	if !json.Valid(raw) {
		return SourceContent{}, ErrInvalidInput
	}
	now := s.now()
	item, err := s.store.CreateSource(SourceContent{TeamID: *team, Platform: strings.TrimSpace(input.Platform), PlatformContentID: strings.TrimSpace(input.PlatformContentID), Title: input.Title, Description: input.Description, CoverURL: input.CoverURL, SourceURL: input.SourceURL, AuthorID: input.AuthorID, AuthorName: input.AuthorName, SourceType: input.SourceType, PublishedAt: input.PublishedAt, Status: StatusPending, CreatedBy: actor.ID, CreatedAt: now, UpdatedAt: now}, raw)
	if err != nil {
		if errors.Is(err, ErrDuplicate) {
			return SourceContent{}, ErrDuplicate
		}
		return SourceContent{}, err
	}
	return item, nil
}

func (s *Service) List(actor identity.PublicUser, filter Filter) ([]SourceContent, error) {
	scope, err := s.Scope(actor, filter.TeamID)
	if err != nil {
		return nil, err
	}
	filter.TeamID = scope
	return s.store.ListSources(filter)
}

func (s *Service) Get(actor identity.PublicUser, id int64) (SourceContent, bool, error) {
	item, found, err := s.store.FindSource(id)
	if err != nil || !found {
		return item, found, err
	}
	if _, err := s.Scope(actor, &item.TeamID); err != nil {
		return SourceContent{}, false, err
	}
	return item, true, nil
}

func (s *Service) SetStatus(actor identity.PublicUser, id int64, status Status, reason string) (SourceContent, error) {
	item, found, err := s.Get(actor, id)
	if err != nil {
		return SourceContent{}, err
	}
	if !found {
		return SourceContent{}, ErrNotFound
	}
	if status != StatusIgnored && status != StatusPending {
		return SourceContent{}, ErrInvalidInput
	}
	if item.Status == StatusMaterialCreated && status == StatusPending {
		return SourceContent{}, ErrInvalidTransition
	}
	return s.store.UpdateStatus(id, status, reason)
}

// BatchSetStatus applies each item independently so one invalid item does not
// roll back successful updates from the same operator action.
func (s *Service) BatchSetStatus(actor identity.PublicUser, ids []int64, status Status, reason string) ([]SourceContent, error) {
	if len(ids) == 0 || len(ids) > 500 {
		return nil, ErrInvalidInput
	}
	updated := make([]SourceContent, 0, len(ids))
	for _, id := range ids {
		item, err := s.SetStatus(actor, id, status, reason)
		if err != nil {
			return updated, err
		}
		updated = append(updated, item)
	}
	return updated, nil
}

func (s *Service) Materialize(actor identity.PublicUser, id int64) (Material, error) {
	_, found, err := s.Get(actor, id)
	if err != nil {
		return Material{}, err
	}
	if !found {
		return Material{}, ErrNotFound
	}
	return s.store.Materialize(id, actor.ID, s.now())
}
