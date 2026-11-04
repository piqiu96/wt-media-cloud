package repository

import (
	"github.com/DATA-DOG/go-sqlmock"
	"testing"
)

func TestBatchActionSkipCountsAreAggregatedWithoutLoadingRows(t *testing.T) {
	db, mock := newMockGORM(t)
	mock.ExpectQuery(`SELECT COALESCE\(SUM`).WithArgs(int64(8)).WillReturnRows(sqlmock.NewRows([]string{"uncertain", "wrong_retry_state", "cancel_requested", "terminal"}).AddRow(2, 3, 1, 4))
	retry, cancel, err := batchActionSkips(db, 8)
	if err != nil || len(retry) != 2 || retry[0].Count != 2 || retry[0].Reason != "结果不明确，需人工核实" || retry[1].Count != 3 || len(cancel) != 2 || cancel[0].Count != 1 || cancel[1].Count != 4 {
		t.Fatalf("retry=%+v cancel=%+v err=%v", retry, cancel, err)
	}
	assertExpectations(t, mock)
}
