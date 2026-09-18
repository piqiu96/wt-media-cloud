package repository

import (
	"testing"

	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/model"
	"gorm.io/gorm"
)

func TestMediaAccountRepositoryExposesPackageFunctionsUsingGORM(t *testing.T) {
	var _ func(model.AccountRecord) (string, error) = Create
	var _ func(string) (model.AccountRecord, bool, error) = FindByID
	var _ func(string) error = Delete
	var _ func(*gorm.DB, model.AccountRecord) (string, error) = create
	var _ func(*gorm.DB, string) (model.AccountRecord, bool, error) = findByID
	var _ func(*gorm.DB, string) error = delete
}
