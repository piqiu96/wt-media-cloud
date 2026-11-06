package repository

import (
	"fmt"
	"strings"

	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

func CountMaterials(filter MaterialFilter) (int, error) {
	conditions, args := materialConditions(filter)
	query := "SELECT COUNT(*) FROM materials m JOIN source_contents s ON s.id=m.source_content_id"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	var count int
	err := database.DB().Raw(query, args...).Row().Scan(&count)
	return count, err
}
func OwnerUsagesForMaterials(owner identity.UserID, ids []int64, status string) ([]model.MaterialUsage, error) {
	result := []model.MaterialUsage{}
	if len(ids) == 0 {
		return result, nil
	}
	args := []any{owner}
	for _, id := range ids {
		args = append(args, id)
	}
	query := fmt.Sprintf("SELECT id,team_id,material_id,user_id,status,removed_at,created_at,updated_at FROM material_usages WHERE user_id=? AND material_id IN (%s)", strings.TrimSuffix(strings.Repeat("?,", len(ids)), ","))
	if status == "active" || status == "removed" {
		query += " AND status=?"
		args = append(args, status)
	}
	rows, err := database.DB().Raw(query, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var usage model.MaterialUsage
		if err := scanUsage(rows, &usage); err != nil {
			return nil, err
		}
		result = append(result, usage)
	}
	return result, rows.Err()
}

// MaterialMetricPresence reads preserved provider evidence once for a page.
func MaterialMetricPresence(ids []int64) (map[int64]map[string]bool, error) {
	result := map[int64]map[string]bool{}
	if len(ids) == 0 {
		return result, nil
	}
	args := []any{}
	for _, id := range ids {
		args = append(args, id)
	}
	query := fmt.Sprintf("SELECT m.id,s.raw_json,s.view_count,s.like_count,s.favorite_count,s.comment_count,s.share_count FROM materials m JOIN source_contents s ON s.id=m.source_content_id WHERE m.id IN (%s)", strings.TrimSuffix(strings.Repeat("?,", len(ids)), ","))
	rows, err := database.DB().Raw(query, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, views, likes, favorites, comments, shares int64
		var raw []byte
		if err := rows.Scan(&id, &raw, &views, &likes, &favorites, &comments, &shares); err != nil {
			return nil, err
		}
		result[id] = map[string]bool{"view_count": SourceMetricKnown(raw, "view_count", views), "like_count": SourceMetricKnown(raw, "like_count", likes), "favorite_count": SourceMetricKnown(raw, "favorite_count", favorites), "comment_count": SourceMetricKnown(raw, "comment_count", comments), "share_count": SourceMetricKnown(raw, "share_count", shares)}
	}
	return result, rows.Err()
}
