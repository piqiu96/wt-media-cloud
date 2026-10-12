package service

import (
	"errors"

	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
)

var ErrTemplateForbidden = errors.New("compose template catalog is forbidden")

type TemplateStore interface {
	ListActiveTemplates() ([]model.ComposeTemplate, error)
}

type TemplateService struct {
	store TemplateStore
}

func NewTemplateService(store TemplateStore) *TemplateService {
	return &TemplateService{store: store}
}

func ListComposeTemplates(actor identityservice.PublicUser) ([]model.ComposeTemplate, error) {
	return NewTemplateService(repository.TemplateStore{}).List(actor)
}

func (s *TemplateService) List(actor identityservice.PublicUser) ([]model.ComposeTemplate, error) {
	if actor.Status != identityservice.UserStatusEnabled {
		return nil, ErrTemplateForbidden
	}
	items, err := s.store.ListActiveTemplates()
	if err != nil {
		return nil, err
	}
	if items == nil {
		return []model.ComposeTemplate{}, nil
	}
	return items, nil
}
