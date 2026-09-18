package repository

import (
	"testing"

	"github.com/wt-media/wt-media-cloud/internal/modules/profileguard/model"
	"gorm.io/gorm"
)

func TestProfileGuardRepositoryExposesPackageFunctionsUsingGORM(t *testing.T) {
	var _ func(string) (model.SensitiveTask, bool, error) = FindAuthorizedTask
	var _ func(*gorm.DB, string) (model.SensitiveTask, bool, error) = findAuthorizedTask
	var _ func(*gorm.DB, model.SensitiveTask) error = createAuthorizedTask
}
