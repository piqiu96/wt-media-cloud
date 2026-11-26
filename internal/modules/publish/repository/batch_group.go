package repository

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"gorm.io/gorm"
)

// A group is assigned once. Free space after assignment is never filled by
// another row. A busy actual Profile is not reserved by group membership.
func (s ExecutionStore) prepareBatchGroups(user int64, cap model.ExecutionCapacity, supported []string, now time.Time) error {
	var after uint64
	for {
		var ids []uint64
		if err := s.DB.Model(&model.Batch{}).
			Where("operator_id=? AND execution_status=? AND started_at IS NOT NULL AND stopped_at IS NULL AND (execution_group_no IS NULL OR (group_ended_at IS NOT NULL AND next_group_at<=?))", user, model.BatchRunning, now).
			Where(`EXISTS (SELECT 1 FROM publication_task pending
			WHERE pending.batch_id=publication_batch.id AND pending.status IN ?
			AND pending.execution_group_no IS NULL AND pending.authorized_at IS NOT NULL
			AND pending.cancel_requested_at IS NULL AND (pending.not_before IS NULL OR pending.not_before<=?))`,
				[]model.Status{model.StatusPending, model.StatusQueued}, now).
			Where("platform IN ?", supported).Where("id>?", after).Order("id").Limit(100).Pluck("id", &ids).Error; err != nil {
			return err
		}
		for _, id := range ids {
			assigned, capacityBlocked := false, false
			err := currentTransaction(s.DB, func(tx *gorm.DB) error {
				b, err := lockBatch(tx, id)
				if err != nil {
					return err
				}
				if !slices.Contains(supported, b.Platform) || stoppedParent(&b) || b.OperatorID != user || b.ExecutionStatus != model.BatchRunning || b.StartedAt == nil || b.StoppedAt != nil ||
					b.ExecutionGroupNo != nil && (b.GroupEndedAt == nil || b.NextGroupAt == nil || now.Before(*b.NextGroupAt)) {
					return nil
				}
				if err = admissionOpen(tx); err != nil {
					return err
				}
				activeAuto, err := holderCount(tx, user, true)
				if err != nil {
					return err
				}
				activeWindows, err := holderCount(tx, user, false)
				if err != nil {
					return err
				}
				wanted := min(b.ExecutionSetting.ConcurrencyLimit, int(cap.AutomaticSlots)-activeAuto, int(cap.WindowSlots)-activeWindows)
				if wanted <= 0 {
					capacityBlocked = true
					return nil
				}
				rows := []model.Task{}
				if err = tx.Where("batch_id=? AND status IN ? AND execution_group_no IS NULL AND authorized_at IS NOT NULL AND cancel_requested_at IS NULL AND (not_before IS NULL OR not_before<=?)",
					b.ID, []model.Status{model.StatusPending, model.StatusQueued}, now).Order("task_order,id").Find(&rows).Error; err != nil {
					return err
				}
				next := uint32(1)
				if b.ExecutionGroupNo != nil {
					next = *b.ExecutionGroupNo + 1
				}
				if next == 0 {
					return errors.New("publication group number overflow")
				}
				seenUID, seenProfile := map[string]bool{}, map[[2]string]bool{}
				selected := []uint64{}
				for _, row := range rows {
					input, e := s.readInput(tx, row)
					if errors.Is(e, ErrConflict) || errors.Is(e, ErrNotFound) {
						continue
					}
					if e != nil {
						return e
					}
					identity := [2]string{input.BitMainUserID, input.BitProfileID}
					uid := row.Platform + "\x00" + input.PlatformUID
					if identity[0] == "" || identity[1] == "" || seenUID[uid] || seenProfile[identity] {
						continue
					}
					var busy int64
					if e = tx.Model(&model.ProfileUse{}).Where("active_main_id=? AND active_profile_id=?", identity[0], identity[1]).Count(&busy).Error; e != nil {
						return e
					}
					if busy > 0 {
						continue
					}
					if e = tx.Model(&model.Task{}).Where("platform=? AND executing_account_uid=?", row.Platform, input.PlatformUID).Count(&busy).Error; e != nil {
						return e
					}
					if busy > 0 {
						continue
					}
					if e = updateTask(tx, row, map[string]any{"execution_group_no": next, "status": model.StatusQueued, "updated_at": now}); e != nil {
						return e
					}
					selected = append(selected, row.ID)
					seenUID[uid], seenProfile[identity] = true, true
					if len(selected) >= wanted {
						break
					}
				}
				if len(selected) == 0 {
					return nil
				}
				b.Revision++
				if err = tx.Model(&model.Batch{}).Where("id=?", b.ID).Updates(map[string]any{
					"execution_group_no": next, "group_ended_at": nil, "next_group_at": nil, "revision": b.Revision, "updated_at": now,
				}).Error; err != nil {
					return err
				}
				assigned = true
				return auditBatchDetails(tx, b, 0, "publication_batch.group_assigned", map[string]any{"revision": b.Revision, "group": next, "task_ids": fmt.Sprint(selected)})
			})
			if err != nil {
				return err
			}
			// ClaimWithCapacity now proceeds to the queued task scan. Assign one
			// fixed group per call; a full global capacity pool waits for the next poll.
			if assigned || capacityBlocked {
				return nil
			}
		}
		if len(ids) < 100 {
			return nil
		}
		// Busy/invalid inputs do not change their batches. Move past this page
		// instead of repeatedly starving a later independent Profile.
		after = ids[len(ids)-1]
	}
}
