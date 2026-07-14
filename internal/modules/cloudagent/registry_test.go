package cloudagent

import (
	"errors"
	"testing"
	"time"
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
