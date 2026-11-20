package repository

import "github.com/wt-media/wt-media-cloud/internal/modules/publish/model"

// Use the same generated ownership column as the formal create constraint.
// Terminal tasks retain ownership; abandoned tasks release it. Actor/list scope
// does not affect whether an output is already held by another task.
func (s Store) OccupiedOutputIDs(platform string, ids []int64) ([]int64, error) {
	occupied := []int64{}
	if len(ids) == 0 {
		return occupied, nil
	}
	err := s.DB.Model(&model.Task{}).Where("platform=? AND occupied_output_id IN ?", platform, ids).Distinct().Pluck("occupied_output_id", &occupied).Error
	return occupied, err
}
