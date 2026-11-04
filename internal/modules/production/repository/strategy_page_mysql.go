package repository

import (
	"context"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"gorm.io/gorm"
)

type ComposeStrategyPage struct {
	Items    []model.ComposeStrategy `json:"items"`
	Total    int64                   `json:"total"`
	Page     int                     `json:"page"`
	PageSize int                     `json:"page_size"`
}

func (StrategyStore) Page(ctx context.Context, f StrategyFilter, page, size int) (ComposeStrategyPage, error) {
	return pageComposeStrategies(database.DB().WithContext(ctx), f, page, size)
}
func pageComposeStrategies(db *gorm.DB, f StrategyFilter, page, size int) (ComposeStrategyPage, error) {
	result := ComposeStrategyPage{Items: []model.ComposeStrategy{}, Page: page, PageSize: size}
	where, args := strategyListWhere(f)
	if err := db.Raw(`SELECT COUNT(*)`+strategyListFrom+where, args...).Row().Scan(&result.Total); err != nil {
		return result, err
	}
	pageArgs := append(append([]any(nil), args...), size, (page-1)*size)
	rows, err := db.Raw(strategyListSelect+strategyListFrom+where+` ORDER BY s.id DESC LIMIT ? OFFSET ?`, pageArgs...).Rows()
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		item, err := scanComposeStrategy(rows)
		if err != nil {
			return result, err
		}
		result.Items = append(result.Items, item)
	}
	return result, rows.Err()
}
