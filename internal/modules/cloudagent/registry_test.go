package cloudagent

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRegistryRegisterAndHeartbeat(t *testing.T) {
	now := time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC)
	registry := NewRegistryWithClock(func() time.Time { return now })

	node, err := registry.Register(RegisterAgentRequest{
		AgentID:              "agent-local-1",
		Mode:                 "local",
		Version:              "0.1.0",
		ContractMajorVersion: MajorVersion,
		ContractRevision:     ContractRevision,
		Capabilities:         []string{"noop"},
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	if node.AgentID != "agent-local-1" || node.Status != AgentStatusOnline {
		t.Fatalf("unexpected node: %+v", node)
	}
	if node.RegisteredAt != "2026-07-14T08:00:00Z" {
		t.Fatalf("RegisteredAt = %q", node.RegisteredAt)
	}

	now = now.Add(30 * time.Second)
	updated, err := registry.Heartbeat("agent-local-1", HeartbeatRequest{Status: AgentStatusDraining})
	if err != nil {
		t.Fatalf("Heartbeat returned error: %v", err)
	}
	if updated.Status != AgentStatusDraining {
		t.Fatalf("Status = %q", updated.Status)
	}
	if updated.LastHeartbeatAt != "2026-07-14T08:00:30Z" {
		t.Fatalf("LastHeartbeatAt = %q", updated.LastHeartbeatAt)
	}
}

func TestRegistryRejectsIncompatibleAgent(t *testing.T) {
	registry := NewRegistry()

	_, err := registry.Register(RegisterAgentRequest{
		AgentID:              "agent-local-1",
		Mode:                 "local",
		Version:              "0.1.0",
		ContractMajorVersion: "v2",
		ContractRevision:     ContractRevision,
	})
	if !errors.Is(err, ErrIncompatibleAgent) {
		t.Fatalf("err = %v", err)
	}
}

func TestRegistryHeartbeatUnknownAgent(t *testing.T) {
	registry := NewRegistry()

	_, err := registry.Heartbeat("missing", HeartbeatRequest{})
	if !errors.Is(err, ErrAgentNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestMySQLRegistryInvalidatedLocalSessionDrainsNode(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	registry := NewMySQLRegistry(db)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*) > 0 FROM local_agent_nodes n
				 JOIN user_sessions s ON n.session_id = s.id
				 WHERE n.agent_id = ? AND n.mode = 'local' AND s.invalidated_at IS NULL`)).
		WithArgs("agent-local-1").
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*) > 0"}).AddRow(false))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE agent_nodes SET status = 'draining' WHERE agent_id = ?`)).
		WithArgs("agent-local-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE local_agent_nodes SET status = 'replaced' WHERE agent_id = ?`)).
		WithArgs("agent-local-1").WillReturnResult(sqlmock.NewResult(0, 1))

	_, err = registry.Heartbeat("agent-local-1", HeartbeatRequest{Status: AgentStatusOnline})
	if !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("Heartbeat() error = %v, want ErrSessionInvalid", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
