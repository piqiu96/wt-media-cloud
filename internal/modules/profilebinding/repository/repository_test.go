package repository

import (
	"testing"

	"github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/model"
	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
	"gorm.io/gorm"
)

func TestProfileBindingRepositoryExposesPackageFunctionsUsingGORM(t *testing.T) {
	var _ func(sharedidentity.UserID) (model.BitAccountBinding, bool, error) = FindBinding
	var _ func(model.ProfileScan) error = CreateScan
	var _ func(string) (model.ProfileScan, bool, error) = FindScan
	var _ func(string) (model.BrowserProfile, bool, error) = GetProfile

	var _ func(*gorm.DB, sharedidentity.UserID) (model.BitAccountBinding, bool, error) = findBinding
	var _ func(*gorm.DB, model.ProfileScan) error = createScan
	var _ func(*gorm.DB, string) (model.ProfileScan, bool, error) = findScan
	var _ func(*gorm.DB, string) (model.BrowserProfile, bool, error) = getProfile
}
