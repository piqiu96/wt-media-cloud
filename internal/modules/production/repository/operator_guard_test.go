package repository

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
)

func TestComposeOperatorAvailableChecksEnabledTeamGameResponsibility(t *testing.T) {
	db, mock := newMockGORM(t)
	operatorID := int64(9)
	mock.ExpectQuery(`FROM users u JOIN user_game_scopes s ON s.user_id = u.id`).
		WithArgs(operatorID, int64(7), "delta").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	ok, err := composeOperatorAvailable(db, model.ComposeStrategy{TeamID: 7, GameID: "delta", OperatorID: &operatorID})
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	assertExpectations(t, mock)

	db, mock = newMockGORM(t)
	mock.ExpectQuery(`FROM users u JOIN user_game_scopes s`).
		WithArgs(operatorID, int64(7), "delta").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	ok, err = composeOperatorAvailable(db, model.ComposeStrategy{TeamID: 7, GameID: "delta", OperatorID: &operatorID})
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	assertExpectations(t, mock)
}

func TestComposeOperatorAvailableRefusesUnassignedWithoutAQuery(t *testing.T) {
	db, mock := newMockGORM(t)
	ok, err := composeOperatorAvailable(db, model.ComposeStrategy{TeamID: 7, GameID: "delta"})
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	assertExpectations(t, mock)
}
