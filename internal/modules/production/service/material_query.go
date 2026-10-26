package service

import (
	"sort"
	"strconv"
	"strings"
	"time"

	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
)

// MyMaterialsQuery is applied on the Cloud projection after access control and
// per-user download status have been resolved.
type MyMaterialsQuery struct {
	Search      string
	VideoStatus string
	SortBy      string
	SortOrder   string
}

func ListMyMaterialsFiltered(actor identityservice.PublicUser, query MyMaterialsQuery) ([]model.MaterialUsage, error) {
	return newWiredService().ListMyMaterialsFiltered(actor, query)
}

func (s *Service) ListMyMaterialsFiltered(actor identityservice.PublicUser, query MyMaterialsQuery) ([]model.MaterialUsage, error) {
	items, err := s.ListMyMaterials(actor)
	if err != nil {
		return nil, err
	}
	search := strings.ToLower(strings.TrimSpace(query.Search))
	filtered := make([]model.MaterialUsage, 0, len(items))
	for _, usage := range items {
		material := usage.Material
		if material == nil {
			continue
		}
		if query.VideoStatus != "" && string(material.VideoStatus) != query.VideoStatus {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(material.Title+" "+material.AuthorName), search) && search != formatMaterialID(material.ID) {
			continue
		}
		filtered = append(filtered, usage)
	}
	ascending := strings.EqualFold(query.SortOrder, "asc")
	sort.SliceStable(filtered, func(i, j int) bool {
		a, b := filtered[i], filtered[j]
		compare := 0
		switch query.SortBy {
		case "id":
			compare = compareInt64(a.MaterialID, b.MaterialID)
		case "added_at":
			compare = compareTime(a.CreatedAt, b.CreatedAt)
		case "created_at":
			compare = compareTime(a.Material.CreatedAt, b.Material.CreatedAt)
		default:
			compare = compareOptionalTime(a.Material.PublishedAt, b.Material.PublishedAt)
		}
		if compare == 0 {
			return a.ID > b.ID
		}
		if ascending {
			return compare < 0
		}
		return compare > 0
	})
	return filtered, nil
}

func formatMaterialID(id int64) string { return strconv.FormatInt(id, 10) }
func compareInt64(a, b int64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
func compareTime(a, b time.Time) int {
	if a.Before(b) {
		return -1
	}
	if a.After(b) {
		return 1
	}
	return 0
}
func compareOptionalTime(a, b *time.Time) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}
	return compareTime(*a, *b)
}
