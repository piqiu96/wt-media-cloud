// Package service owns material visibility and the “My Materials” use case.
package service

import (
	"errors"
	"strings"
	"time"

	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
)

var (
	ErrInvalidInput = errors.New("production input is invalid")
	ErrForbidden    = errors.New("production operation is forbidden")
	ErrNotFound     = errors.New("material not found")
)

type Store interface {
	FindMaterial(int64) (model.Material, bool, error)
	ListMaterials(repository.MaterialFilter) ([]model.Material, error)
	CreateOrRestoreUsage(repository.CreateUsageInput, time.Time) (model.MaterialUsage, error)
}

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service { return &Service{store: store, now: time.Now} }

func (s *Service) GetMaterial(actor identityservice.PublicUser, materialID int64) (model.Material, error) {
	if materialID <= 0 {
		return model.Material{}, ErrInvalidInput
	}
	material, found, err := s.store.FindMaterial(materialID)
	if err != nil {
		return model.Material{}, err
	}
	if !found {
		return model.Material{}, ErrNotFound
	}
	if !canAccessMaterial(actor, material) {
		return model.Material{}, ErrForbidden
	}
	return material, nil
}

func (s *Service) AddUsage(actor identityservice.PublicUser, materialID int64) (model.MaterialUsage, error) {
	material, err := s.GetMaterial(actor, materialID)
	if err != nil {
		return model.MaterialUsage{}, err
	}
	usage, err := s.store.CreateOrRestoreUsage(repository.CreateUsageInput{TeamID: material.TeamID, MaterialID: material.ID, UserID: actor.ID}, s.now().UTC())
	if err != nil {
		return model.MaterialUsage{}, err
	}
	return usage, nil
}

func (s *Service) ListMaterials(actor identityservice.PublicUser, search string) ([]model.Material, error) {
	if actor.Status != identityservice.UserStatusEnabled {
		return nil, ErrForbidden
	}
	filter := repository.MaterialFilter{Search: strings.TrimSpace(search)}
	if actor.Role != identityservice.RoleAdmin {
		if actor.TeamID == nil || len(actor.GameIDs) == 0 {
			return nil, ErrForbidden
		}
		filter.TeamID = actor.TeamID
		filter.GameIDs = append([]string(nil), actor.GameIDs...)
	}
	return s.store.ListMaterials(filter)
}

func canAccessMaterial(actor identityservice.PublicUser, material model.Material) bool {
	if actor.Status != identityservice.UserStatusEnabled {
		return false
	}
	if actor.Role == identityservice.RoleAdmin {
		return true
	}
	if actor.TeamID == nil || *actor.TeamID != material.TeamID || material.GameID == nil {
		return false
	}
	for _, gameID := range actor.GameIDs {
		if strings.TrimSpace(gameID) == *material.GameID {
			return true
		}
	}
	return false
}
