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

	// ErrUsageNotFound is a missing "My Materials" row, which is a different
	// answer from a missing material: the two carry different errcodes and
	// different copy in the UI. It is a separate sentinel rather than a wrapper
	// around ErrNotFound precisely so the handler can keep them apart.
	ErrUsageNotFound = errors.New("material usage not found")
)

type Store interface {
	FindMaterial(int64) (model.Material, bool, error)
	ListMaterials(repository.MaterialFilter) ([]model.Material, error)
	CreateOrRestoreUsage(repository.CreateUsageInput, time.Time) (model.MaterialUsage, error)
	ListActiveUsages(identityservice.UserID) ([]model.MaterialUsage, error)
	FindUsageForUser(int64, identityservice.UserID) (model.MaterialUsage, bool, error)
	RemoveUsageByID(int64, identityservice.UserID, time.Time) (bool, error)
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

func (s *Service) ListMyMaterials(actor identityservice.PublicUser) ([]model.MaterialUsage, error) {
	if actor.Status != identityservice.UserStatusEnabled || actor.ID <= 0 {
		return nil, ErrForbidden
	}
	usages, err := s.store.ListActiveUsages(actor.ID)
	if err != nil {
		return nil, err
	}
	visible := make([]model.MaterialUsage, 0, len(usages))
	for _, usage := range usages {
		material, found, err := s.store.FindMaterial(usage.MaterialID)
		if err != nil {
			return nil, err
		}
		if !found || !canAccessMaterial(actor, material) {
			continue
		}
		usage.Material = &material
		visible = append(visible, usage)
	}
	return visible, nil
}

func (s *Service) RemoveUsage(actor identityservice.PublicUser, usageID int64) error {
	if usageID <= 0 || actor.ID <= 0 {
		return ErrInvalidInput
	}
	usage, found, err := s.store.FindUsageForUser(usageID, actor.ID)
	if err != nil {
		return err
	}
	if !found {
		return ErrUsageNotFound
	}
	material, err := s.GetMaterial(actor, usage.MaterialID)
	if err != nil {
		return err
	}
	if material.TeamID != usage.TeamID {
		return ErrForbidden
	}
	removed, err := s.store.RemoveUsageByID(usageID, actor.ID, s.now().UTC())
	if err != nil {
		return err
	}
	if !removed {
		// The row left the actor's scope between the lookup and the update — the
		// conditional `status = 'active'` in the same statement is what reports
		// it. That is the same outcome as never having had it.
		return ErrUsageNotFound
	}
	return nil
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
