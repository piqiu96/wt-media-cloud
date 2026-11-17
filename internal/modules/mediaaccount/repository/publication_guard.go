package repository

import (
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/model"
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
		return model.ErrCookieConflict
	}
	return nil
}
