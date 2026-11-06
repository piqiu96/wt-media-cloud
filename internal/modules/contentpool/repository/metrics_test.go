package repository

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/wt-media/wt-media-cloud/internal/modules/contentpool/model"
)

func TestSourceCaptureTimestampAndDuplicateRefresh(t *testing.T) {
	for _, duplicate := range []bool{false, true} {
		t.Run(map[bool]string{false: "first capture", true: "refresh existing"}[duplicate], func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			captured := time.Date(2026, 11, 5, 1, 0, 0, 0, time.UTC)
			v := model.SourceContent{TeamID: 7, Platform: "douyin", PlatformContentID: "123", MetricsUpdatedAt: &captured, LikeCount: 12, FavoriteCount: 3, ViewCount: 50, CommentCount: 2, ShareCount: 1, UpdatedAt: captured}
			insert := mock.ExpectExec(`INSERT INTO source_contents .*metrics_updated_at`)
			if duplicate {
				insert.WillReturnError(&mysql.MySQLError{Number: 1062})
				mock.ExpectExec(regexp.QuoteMeta(refreshSourceMetricsSQL)).WithArgs(int64(12), int64(3), int64(50), int64(2), int64(1), []byte(`{}`), &captured, captured, v.TeamID, "douyin", "123", &captured).WillReturnResult(sqlmock.NewResult(0, 1))
			} else {
				insert.WillReturnResult(sqlmock.NewResult(9, 1))
				mock.ExpectQuery("SELECT LAST_INSERT_ID").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
			}
			item, err := createSource(testGORM(db), v, []byte(`{}`))
			if duplicate {
				if !errors.Is(err, errDuplicate) {
					t.Fatalf("duplicate semantics changed: %v", err)
				}
			} else if err != nil || item.MetricsUpdatedAt == nil || !item.MetricsUpdatedAt.Equal(captured) {
				t.Fatalf("capture not preserved: %+v %v", item, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
