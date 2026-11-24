package repository

import (
	"errors"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func admissionOpen(tx *gorm.DB) error {
	var gate struct{ VerifiedAt *time.Time }
	err := tx.Table("publication_admission_cutover").Clauses(clause.Locking{Strength: "SHARE"}).Select("verified_at").Where("id=1").Take(&gate).Error
	if err != nil {
		return err
	}
	if gate.VerifiedAt == nil {
		return ErrAdmissionClosed
	}
	return nil
}

func freeSlot(occupied []uint16, limit uint16) uint16 {
	if len(occupied) >= int(limit) {
		return 0
	}
	used := make(map[uint16]bool, len(occupied))
	for _, slot := range occupied {
		used[slot] = true
	}
	for i := 1; i <= int(limit); i++ {
		if !used[uint16(i)] {
			return uint16(i)
		}
	}
	return 0
}

func freeAutomaticSlot(tx *gorm.DB, owner int64, limit uint16) (uint16, error) {
	var slots []uint16
	err := tx.Model(&model.Attempt{}).Where("executor_user_id=? AND active_automatic_slot IS NOT NULL", owner).Pluck("active_automatic_slot", &slots).Error
	return freeSlot(slots, limit), err
}

func freeWindowSlot(tx *gorm.DB, owner int64, limit uint16) (uint16, error) {
	var slots []uint16
	err := tx.Model(&model.ProfileUse{}).Where("owner_id=? AND released_at IS NULL", owner).Pluck("window_slot", &slots).Error
	return freeSlot(slots, limit), err
}

func duplicateIndex(err error, name string) bool {
	var duplicate *mysql.MySQLError
	return errors.As(err, &duplicate) && duplicate.Number == 1062 && strings.Contains(duplicate.Message, name)
}

func tryProfileUse(tx *gorm.DB, owner int64, task model.Task, in model.ExecutionInput, kind string, limit uint16, now time.Time) (model.ProfileUse, error) {
	v := model.ProfileUse{BitMainUserID: in.BitMainUserID, BitProfileID: in.BitProfileID, OwnerID: owner, PublicationID: task.ID, Kind: kind, AcquiredAt: now}
	if strings.TrimSpace(in.BitMainUserID) == "" || strings.TrimSpace(in.BitProfileID) == "" {
		return v, ErrInputChanged
	}
	slot, err := freeWindowSlot(tx, owner, limit)
	if err != nil {
		return v, err
	}
	if slot == 0 {
		return v, ErrCapacityWait
	}
	v.WindowSlot = slot
	err = tx.Create(&v).Error
	if duplicateIndex(err, "uq_publication_profile_use") {
		return v, ErrProfileUse
	}
	return v, err
}

func lockPublishUse(tx *gorm.DB, attempt uint64) error {
	var use model.ProfileUse
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("attempt_id=? AND kind='publish' AND released_at IS NULL", attempt).Take(&use).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	return err
}

func releasePublishUse(tx *gorm.DB, attempt uint64, now time.Time) error {
	return tx.Model(&model.ProfileUse{}).Where("attempt_id=? AND kind='publish' AND released_at IS NULL", attempt).
		Updates(map[string]any{"released_at": now, "disposition": "closed"}).Error
}

func batchAutomaticAvailable(tx *gorm.DB, b *model.Batch) (bool, error) {
	if b == nil {
		return true, nil
	}
	var n int64
	err := tx.Table("publication_attempt a").Joins("JOIN publication_task t ON t.id=a.publication_id").
		Where("t.batch_id=? AND a.active_automatic_slot IS NOT NULL", b.ID).Count(&n).Error
	return n < int64(b.ExecutionSetting.ConcurrencyLimit), err
}

func holderCount(tx *gorm.DB, owner int64, automatic bool) (int, error) {
	var n int64
	var err error
	if automatic {
		err = tx.Model(&model.Attempt{}).Where("executor_user_id=? AND active_automatic_slot IS NOT NULL", owner).Count(&n).Error
	} else {
		err = tx.Model(&model.ProfileUse{}).Where("owner_id=? AND released_at IS NULL", owner).Count(&n).Error
	}
	return int(n), err
}

func (s ExecutionStore) localCapacity(agent model.ExecutionCapacity) model.ExecutionCapacity {
	configured := s.Capacity
	if !configured.Valid() {
		configured = model.ExecutionCapacity{AutomaticSlots: 5, WindowSlots: 10}
	}
	return configured.Limit(agent)
}
