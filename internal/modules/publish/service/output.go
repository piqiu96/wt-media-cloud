package service

import (
	"context"
	"errors"
	"slices"

	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	outputmodel "github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	production "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
)

// OutputPage is the production service read boundary, without a repository dependency.
type OutputPage struct {
	Items          []outputmodel.CompositeOutput
	Total          int64
	Page, PageSize int
}
type OutputCandidate struct {
	outputmodel.CompositeOutput
	CompositeOutputID int64 `json:"composite_output_id,string"`
	Occupied          bool  `json:"occupied"`
}
type OutputCandidatePage struct {
	Items    []OutputCandidate `json:"items"`
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
}
type outputOccupancyReader interface {
	OccupiedOutputIDs(string, []int64) ([]int64, error)
}

func (s *Service) OutputCandidates(ctx context.Context, a identity.PublicUser, in production.ListComposeOutputsInput) (OutputCandidatePage, error) {
	empty := OutputCandidatePage{}
	if in.Platform == "" {
		return empty, ErrInvalid
	}
	if err := validateFilter(a, Filter{Page: in.Page, PageSize: in.PageSize, Search: in.Search, Platform: in.Platform, CategoryID: int64(in.CategoryID)}); err != nil {
		return empty, err
	}
	if s.OutputPage == nil {
		return empty, errors.New("output candidate provider unavailable")
	}
	// Production still enforces owner and team scope. Senior operators also
	// need a publication category constraint before production counts rows.
	if in.CategoryID > 0 && !a.CanAccess(a.ID, a.TeamID, in.CategoryID) {
		return empty, ErrNotFound
	}
	if a.Role == identity.RoleSeniorOperator {
		allowed := make([]identity.CategoryID, 0, len(a.CategoryIDs))
		for _, category := range a.CategoryIDs {
			if len(in.CategoryIDs) == 0 || slices.Contains(in.CategoryIDs, category) {
				allowed = append(allowed, category)
			}
		}
		if len(allowed) == 0 {
			return OutputCandidatePage{Items: []OutputCandidate{}, Page: in.Page, PageSize: in.PageSize}, nil
		}
		in.CategoryIDs = allowed
	}
	page, err := s.OutputPage(ctx, a, in)
	if err != nil {
		return empty, err
	}
	result := OutputCandidatePage{Items: []OutputCandidate{}, Total: page.Total, Page: page.Page, PageSize: page.PageSize}
	if len(page.Items) == 0 {
		return result, nil
	}
	ids := make([]int64, 0, len(page.Items))
	for _, item := range page.Items {
		ids = append(ids, item.ID)
	}
	reader, ok := s.Repo.(outputOccupancyReader)
	if !ok {
		return empty, errors.New("output occupancy reader unavailable")
	}
	occupied, err := reader.OccupiedOutputIDs(in.Platform, ids)
	if err != nil {
		return empty, err
	}
	used := make(map[int64]bool, len(occupied))
	for _, id := range occupied {
		used[id] = true
	}
	for _, item := range page.Items {
		result.Items = append(result.Items, OutputCandidate{CompositeOutput: item, CompositeOutputID: item.ID, Occupied: used[item.ID]})
	}
	return result, nil
}
