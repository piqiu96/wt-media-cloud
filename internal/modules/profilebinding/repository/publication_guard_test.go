package repository

import (
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/model"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestPublicationGuardLocksAndRefusesMutationBeforeUpdate(t *testing.T) {
	unavailable := errors.New("database unavailable")
	for _, tc := range []struct {
		name      string
		busy      bool
		err, want error
	}{{"active scene", true, nil, model.ErrProfileReferenced}, {"lookup failure", false, unavailable, unavailable}} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, closeDB := newProfileMockGORM(t)
			defer closeDB()
			SetPublicationGuard(func(tx *gorm.DB, id string) (bool, error) {
				if tx == db || id != "37" {
					t.Fatal("guard must share locked transaction")
				}
				return tc.busy, tc.err
			})
			defer SetPublicationGuard(nil)
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT id FROM browser_profiles WHERE id").WithArgs("37").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("37"))
			mock.ExpectRollback()
			enabled := false
			e := updateProfile(db, "37", nil, &enabled, time.Now())
			if !errors.Is(e, tc.want) {
				t.Fatalf("%v", e)
			}
		})
	}
}
