package service

import (
	"context"
	"fmt"
	"strings"

	transfers "github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/service"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
)

type MaterialPage struct {
	Items    []model.Material `json:"items"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}
type MaterialUsagePage struct {
	Items    []model.MaterialUsage `json:"items"`
	Total    int                   `json:"total"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
}
type MaterialSelectorContext struct {
	StrategyID int64 `json:"strategy_id"`
	OperatorID int64 `json:"operator_id"`
}

type materialPageStore interface {
	MaterialMetricPresence([]int64) (map[int64]map[string]bool, error)
	CountMaterials(repository.MaterialFilter) (int, error)
	OwnerUsagesForMaterials(identity.UserID, []int64, string) ([]model.MaterialUsage, error)
	MaterialHistory(int64, []int64) ([]model.ComposeMaterialState, error)
}

func (mysqlStore) MaterialMetricPresence(ids []int64) (map[int64]map[string]bool, error) {
	return repository.MaterialMetricPresence(ids)
}
func (mysqlStore) CountMaterials(f repository.MaterialFilter) (int, error) {
	return repository.CountMaterials(f)
}
func (mysqlStore) OwnerUsagesForMaterials(owner identity.UserID, ids []int64, status string) ([]model.MaterialUsage, error) {
	return repository.OwnerUsagesForMaterials(owner, ids, status)
}
func (mysqlStore) MaterialHistory(strategy int64, ids []int64) ([]model.ComposeMaterialState, error) {
	return (repository.ComposeTaskStore{}).MaterialStates(strategy, ids)
}

func (s *Service) materialPageScope(actor identity.PublicUser, f repository.MaterialFilter, mine bool) (repository.MaterialFilter, identity.UserID, error) {
	if actor.Status != identity.UserStatusEnabled || actor.ID <= 0 {
		return f, 0, ErrForbidden
	}
	owner := actor.ID
	if f.StrategyID > 0 {
		lookup := s.selectorStrategy
		if lookup == nil {
			lookup = (repository.StrategyStore{}).Find
		}
		strategy, found, err := lookup(f.StrategyID)
		if err != nil {
			return f, 0, err
		}
		if !found || !canReadStrategy(actor, strategy) || strategy.Status != model.ComposeStrategyEnabled || strategy.OperatorID == nil {
			return f, 0, ErrForbidden
		}
		if f.OperatorID <= 0 || f.OperatorID != *strategy.OperatorID {
			return f, 0, ErrForbidden
		}
		if f.TeamID != nil && int64(*f.TeamID) != strategy.TeamID {
			return f, 0, ErrForbidden
		}
		if f.CategoryID > 0 && f.CategoryID != strategy.CategoryID {
			return f, 0, ErrForbidden
		}
		for _, id := range f.CategoryIDs {
			if id != strategy.CategoryID {
				return f, 0, ErrForbidden
			}
		}
		if _, ok, err := resolveComposeOperator(s.operators, strategy.TeamID, strategy.CategoryID, *strategy.OperatorID); err != nil {
			return f, 0, err
		} else if !ok {
			return f, 0, ErrForbidden
		}
		owner = identity.UserID(*strategy.OperatorID)
		teamID := identity.TeamID(strategy.TeamID)
		f.TeamID = &teamID
		f.CategoryID = strategy.CategoryID
		f.CategoryIDs = nil
	} else {
		if f.OperatorID != 0 {
			return f, 0, ErrForbidden
		}
		switch actor.Role {
		case identity.RoleAdmin:
		case identity.RoleSeniorOperator, identity.RoleOperator:
			if actor.TeamID == nil || (f.TeamID != nil && *f.TeamID != *actor.TeamID) {
				return f, 0, ErrForbidden
			}
			f.TeamID = actor.TeamID
		default:
			return f, 0, ErrForbidden
		}
	}
	if f.UsageStatus != "" && f.UsageStatus != "all" && f.UsageStatus != "active" && f.UsageStatus != "removed" {
		return f, 0, ErrInvalidInput
	}
	if f.StrategyID > 0 {
		f.UsageStatus = "active"
	} else if f.UsageStatus == "" {
		f.UsageStatus = "all"
	}
	f.UserID = actor.ID
	if mine {
		f.OwnerID = owner
	}
	if f.Page == 0 {
		f.Page = 1
	}
	if f.PageSize == 0 {
		f.PageSize = 20
	}
	if f.Page < 1 || f.PageSize < 1 || f.PageSize > 200 {
		return f, 0, fmt.Errorf("%w: pagination invalid", ErrInvalidInput)
	}
	if f.PublishedFrom != nil && f.PublishedBefore != nil && !f.PublishedFrom.Before(*f.PublishedBefore) {
		return f, 0, ErrInvalidInput
	}
	for _, r := range []struct{ min, max *int64 }{{f.ViewCountMin, f.ViewCountMax}, {f.LikeCountMin, f.LikeCountMax}, {f.FavoriteCountMin, f.FavoriteCountMax}} {
		if r.min != nil && *r.min < 0 || r.max != nil && *r.max < 0 || r.min != nil && r.max != nil && *r.min > *r.max {
			return f, 0, ErrInvalidInput
		}
	}
	return f, owner, nil
}
func ListMaterialPage(actor identity.PublicUser, f repository.MaterialFilter) (MaterialPage, error) {
	return newWiredService().ListMaterialPage(actor, f, false)
}
func ListMyMaterialPage(actor identity.PublicUser, f repository.MaterialFilter) (MaterialUsagePage, error) {
	return newWiredService().ListMyMaterialPage(actor, f)
}
func (s *Service) ListMyMaterialPage(actor identity.PublicUser, f repository.MaterialFilter) (MaterialUsagePage, error) {
	page, err := s.ListMaterialPage(actor, f, true)
	if err != nil {
		return MaterialUsagePage{}, err
	}
	scoped, owner, err := s.materialPageScope(actor, f, true)
	if err != nil {
		return MaterialUsagePage{}, err
	}
	ids := []int64{}
	byID := map[int64]model.Material{}
	for _, m := range page.Items {
		ids = append(ids, m.ID)
		byID[m.ID] = m
	}
	rows, err := s.store.(materialPageStore).OwnerUsagesForMaterials(owner, ids, scoped.UsageStatus)
	if err != nil {
		return MaterialUsagePage{}, err
	}
	byMaterial := map[int64]model.MaterialUsage{}
	for _, row := range rows {
		byMaterial[row.MaterialID] = row
	}
	result := MaterialUsagePage{Items: []model.MaterialUsage{}, Total: page.Total, Page: page.Page, PageSize: page.PageSize}
	statuses := map[int64]string{}
	if f.StrategyID == 0 && s.transfers != nil {
		statuses, err = s.transfers.LatestUserDownloadStatuses(actor.ID, ids)
		if err != nil {
			return MaterialUsagePage{}, err
		}
	}
	for _, m := range page.Items {
		if row, ok := byMaterial[m.ID]; ok {
			row.DownloadStatus = statuses[m.ID]
			material := byID[m.ID]
			row.Material = &material
			result.Items = append(result.Items, row)
		}
	}
	return result, nil
}
func (s *Service) ListMaterialPage(actor identity.PublicUser, f repository.MaterialFilter, mine bool) (MaterialPage, error) {
	f, _, err := s.materialPageScope(actor, f, mine)
	if err != nil {
		return MaterialPage{}, err
	}
	store, ok := s.store.(materialPageStore)
	if !ok {
		return MaterialPage{}, fmt.Errorf("material pagination repository unavailable")
	}
	total, err := store.CountMaterials(f)
	if err != nil {
		return MaterialPage{}, err
	}
	items, err := s.store.ListMaterials(f)
	if err != nil {
		return MaterialPage{}, err
	}
	items, err = s.decorateMaterials(actor, items)
	if err != nil {
		return MaterialPage{}, err
	}
	if f.StrategyID > 0 {
		ids := []int64{}
		for _, m := range items {
			ids = append(ids, m.ID)
		}
		presence, err := store.MaterialMetricPresence(ids)
		if err != nil {
			return MaterialPage{}, err
		}
		history, err := store.MaterialHistory(f.StrategyID, ids)
		if err != nil {
			return MaterialPage{}, err
		}
		byID := map[int64]model.ComposeMaterialState{}
		for _, h := range history {
			byID[h.MaterialID] = h
		}
		for i := range items {
			m := &items[i]
			m.MetricPresence = presence[m.ID]
			selectable, reason, err := s.materialSelectionAvailable(*m)
			if err != nil {
				return MaterialPage{}, err
			}
			m.Selectable = &selectable
			m.SelectionReason = reason
			h := byID[m.ID]
			h.MaterialID = m.ID
			m.StrategyHistory = &h
		}
	}
	return MaterialPage{Items: items, Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}
func (s *Service) materialSelectionAvailable(m model.Material) (bool, string, error) {
	if m.Status != model.MaterialAvailable {
		return false, "material_unavailable", nil
	}
	if strings.TrimSpace(m.SourceURL) != "" {
		return true, "", nil
	}
	if m.SourceObjectKey != "" && s.links != nil {
		exists, err := s.links.ObjectExists(context.Background(), m.SourceObjectKey)
		if err != nil {
			return false, "", err
		}
		if exists {
			return true, "", nil
		}
	}
	return false, "source_unavailable", nil
}
func AddUsageForSelection(actor identity.PublicUser, id int64, selection MaterialSelectorContext) (model.MaterialUsage, bool, error) {
	return newWiredService().AddUsageForSelection(actor, id, selection)
}
func (s *Service) AddUsageForSelection(actor identity.PublicUser, id int64, selection MaterialSelectorContext) (model.MaterialUsage, bool, error) {
	if selection.StrategyID <= 0 {
		return model.MaterialUsage{}, false, ErrForbidden
	}
	f, owner, err := s.materialPageScope(actor, repository.MaterialFilter{StrategyID: selection.StrategyID, OperatorID: selection.OperatorID}, true)
	if err != nil {
		return model.MaterialUsage{}, false, err
	}
	material, err := s.getMaterial(actor, id)
	if err != nil {
		return model.MaterialUsage{}, false, err
	}
	if material.TeamID != *f.TeamID || material.CategoryID == nil || *material.CategoryID != f.CategoryID {
		return model.MaterialUsage{}, false, ErrForbidden
	}
	available, _, err := s.materialSelectionAvailable(material)
	if err != nil {
		return model.MaterialUsage{}, false, err
	}
	if !available {
		return model.MaterialUsage{}, false, ErrMaterialUnavailable
	}
	usage, created, err := s.store.CreateOrRestoreUsage(repository.CreateUsageInput{TeamID: material.TeamID, MaterialID: id, UserID: owner}, s.now().UTC())
	if err != nil {
		return model.MaterialUsage{}, false, err
	}
	prepared := false
	if material.SourceObjectKey != "" && s.links != nil {
		prepared, err = s.links.ObjectExists(context.Background(), material.SourceObjectKey)
		if err != nil {
			return model.MaterialUsage{}, false, err
		}
	}
	if !prepared {
		if s.transfers == nil {
			return model.MaterialUsage{}, false, ErrMaterialUnavailable
		}
		_, err = s.transfers.EnsureMaterialSourcePrepare(transfers.EnsureMaterialSourcePrepareInput{TeamID: material.TeamID, AssetID: id, AssetTitle: material.Title, RequestedBy: actor.ID})
		if err != nil {
			return model.MaterialUsage{}, false, err
		}
		if material.VideoStatus == model.VideoReady {
			_, err = s.store.MarkVideoRepreparing(material.TeamID, id, material.SourceObjectKey, material.VideoPreparedAt, s.now().UTC())
		} else {
			_, err = s.store.MarkVideoPreparing(material.TeamID, id, s.now().UTC())
		}
		if err != nil {
			return model.MaterialUsage{}, false, err
		}
	}
	return usage, created, nil
}
