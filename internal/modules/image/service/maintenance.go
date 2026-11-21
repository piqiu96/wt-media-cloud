package service

import (
	"errors"
	files "github.com/wt-media/wt-media-cloud/internal/modules/files/service"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/image/model"
	"gorm.io/gorm"
)

var referenceCheck func(*gorm.DB, int64) error
var listReference func(identity.PublicUser, int64) ([]model.Reference, error)

// SetReferenceProvider binds known business owners during bootstrap.
func SetReferenceProvider(check func(*gorm.DB, int64) error, list func(identity.PublicUser, int64) ([]model.Reference, error)) {
	referenceCheck, listReference = check, list
}
func (s *Service) ListImageAssetReference(a identity.PublicUser, id uint64) ([]model.Reference, error) {
	v, e := s.GetImageAsset(a, id)
	if e != nil {
		return nil, e
	}
	if s.ListReference == nil {
		return nil, errors.New("image reference provider is not configured")
	}
	return s.ListReference(a, v.FileID)
}
func (s *Service) DeleteImageAsset(a identity.PublicUser, id, revision uint64) error {
	if _, e := s.GetImageAsset(a, id); e != nil {
		return e
	}
	if revision == 0 {
		return ErrInvalid
	}
	return s.Repo.Delete(id, revision, s.ReferenceCheck)
}

type BatchItem struct {
	ID               uint64 `json:"id,string"`
	ExpectedRevision uint64 `json:"expected_revision,string"`
}
type BatchInput struct {
	Operation string      `json:"operation"`
	Items     []BatchItem `json:"items"`
}
type BatchResult struct {
	ID      uint64            `json:"id,string"`
	Success bool              `json:"success"`
	Error   string            `json:"error,omitempty"`
	Item    *model.ImageAsset `json:"item,omitempty"`
}

func (s *Service) BatchImageAsset(a identity.PublicUser, in BatchInput) ([]BatchResult, error) {
	if in.Operation != "enable" && in.Operation != "disable" && in.Operation != "dual_usage" || len(in.Items) == 0 || len(in.Items) > 100 {
		return nil, ErrInvalid
	}
	seen := map[uint64]bool{}
	for _, item := range in.Items {
		if item.ID == 0 || item.ExpectedRevision == 0 || seen[item.ID] {
			return nil, ErrInvalid
		}
		seen[item.ID] = true
	}
	out := make([]BatchResult, 0, len(in.Items))
	for _, item := range in.Items {
		v, e := s.GetImageAsset(a, item.ID)
		if e == nil {
			if in.Operation == "dual_usage" {
				v, e = s.UpdateImageAsset(a, item.ID, UpdateInput{ExpectedRevision: item.ExpectedRevision, Name: v.Name, AllowAsCover: true, AllowAsBackground: true, CategoryIDs: v.CategoryIDs})
			} else {
				enabled := in.Operation == "enable"
				v, e = s.SetImageAssetUsage(a, item.ID, UsageInput{ExpectedRevision: item.ExpectedRevision, Enabled: &enabled})
			}
		}
		result := BatchResult{ID: item.ID, Success: e == nil}
		if e == nil {
			result.Item = &v
		} else {
			switch {
			case errors.Is(e, ErrConflict):
				result.Error = "图片已变化，请刷新后重试"
			case errors.Is(e, ErrNotFound):
				result.Error = "图片不存在或没有权限"
			case errors.Is(e, ErrInvalid):
				result.Error = "图片属性不符合要求"
			case errors.Is(e, files.ErrInUse):
				result.Error = "图片被引用"
			default:
				result.Error = "操作未完成，请重试"
			}
		}
		out = append(out, result)
	}
	return out, nil
}
