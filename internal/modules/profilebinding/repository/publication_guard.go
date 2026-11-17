package repository

import (
	"github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/model"
	"gorm.io/gorm"
	"sync"
)

var publicationGuard struct {
	sync.RWMutex
	check func(*gorm.DB, string) (bool, error)
}

func SetPublicationGuard(check func(*gorm.DB, string) (bool, error)) {
	publicationGuard.Lock()
	defer publicationGuard.Unlock()
	publicationGuard.check = check
}
func checkPublication(tx *gorm.DB, id string) error {
	publicationGuard.RLock()
	check := publicationGuard.check
	publicationGuard.RUnlock()
	if check == nil || id == "" {
		return nil
	}
	busy, e := check(tx, id)
	if e != nil {
		return e
	}
	if busy {
		return model.ErrProfileReferenced
	}
	return nil
}

func guardedProfileMutation(db *gorm.DB, id string, work func(*gorm.DB) error) error {
	publicationGuard.RLock()
	enabled := publicationGuard.check != nil
	publicationGuard.RUnlock()
	if !enabled {
		return work(db)
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var locked string
		if e := tx.Raw("SELECT id FROM browser_profiles WHERE id=? FOR UPDATE", id).Row().Scan(&locked); e != nil {
			return e
		}
		if e := checkPublication(tx, id); e != nil {
			return e
		}
		return work(tx)
	})
}
