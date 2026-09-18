package repository

import (
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestGetAgentUsesGORMAndMapsStoredAgent(t *testing.T) {
	db, mock, closeDB := newMockGORM(t)
	defer closeDB()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT agent_id, mode, version")).
		WithArgs("agent-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"agent_id", "mode", "version", "contract_major_version", "contract_revision",
			"status", "registered_at", "last_heartbeat_at", "capabilities",
		}).AddRow("agent-1", "local", "1.0.0", "v1", "2026.07.15.1", "online", "2026-01-01", "2026-01-01", `["proxy"]`))

	agent, err := getAgent(db, "agent-1")
	if err != nil {
		t.Fatalf("getAgent() error = %v", err)
	}
	if agent.AgentID != "agent-1" || len(agent.Capabilities) != 1 || agent.Capabilities[0] != "proxy" {
		t.Fatalf("agent = %#v", agent)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}
