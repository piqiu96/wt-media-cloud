package runtimebinding

import (
	"errors"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
)

type memoryStore struct {
	tickets          map[string]BindingTicket
	nodes            map[string]AgentNode
	activeSessions   map[string]bool
	validProfiles    map[string]bool
	appliedReport    RuntimeReport
	appliedNode      AgentNode
	consumeCount     int
	storedTokenHash  string
	storedCredential string
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		tickets:        make(map[string]BindingTicket),
		nodes:          make(map[string]AgentNode),
		activeSessions: make(map[string]bool),
		validProfiles:  make(map[string]bool),
	}
}

func (s *memoryStore) CreateTicket(ticket BindingTicket) error {
	s.tickets[ticket.TokenHash] = ticket
	s.storedTokenHash = ticket.TokenHash
	return nil
}

func (s *memoryStore) ConsumeTicket(tokenHash string, at time.Time) (BindingTicket, bool, error) {
	ticket, ok := s.tickets[tokenHash]
	if !ok || ticket.UsedAt != nil || !ticket.ExpiresAt.After(at) {
		return BindingTicket{}, false, nil
	}
	usedAt := at
	ticket.UsedAt = &usedAt
	s.tickets[tokenHash] = ticket
	s.consumeCount++
	return ticket, true, nil
}

func (s *memoryStore) IsSessionActive(sessionID, userID string, at time.Time) (bool, error) {
	return s.activeSessions[sessionID+":"+userID], nil
}

func (s *memoryStore) SaveNode(node AgentNode) error {
	s.nodes[node.CredentialHash] = node
	s.storedCredential = node.CredentialHash
	return nil
}

func (s *memoryStore) FindNodeByCredentialHash(hash string) (AgentNode, bool, error) {
	node, ok := s.nodes[hash]
	return node, ok, nil
}

func (s *memoryStore) ValidateRuntimeProfiles(userID, mainUserID string, profileIDs []string) (bool, error) {
	if mainUserID != "main-user-1" {
		return false, nil
	}
	for _, profileID := range profileIDs {
		if !s.validProfiles[userID+":"+profileID] {
			return false, nil
		}
	}
	return true, nil
}

func (s *memoryStore) ApplyRuntimeReport(node AgentNode, report RuntimeReport, at time.Time) error {
	s.appliedNode = node
	s.appliedReport = report
	return nil
}

func testService(store *memoryStore, now *time.Time) *Service {
	ids := 0
	secrets := []string{"binding-secret", "node-secret"}
	return NewService(
		store,
		WithClock(func() time.Time { return *now }),
		WithIDGenerator(func(prefix string) string { ids++; return prefix + "-" + string(rune('0'+ids)) }),
		WithSecretGenerator(func() string { value := secrets[0]; secrets = secrets[1:]; return value }),
	)
}

func TestIssueTicketStoresOnlyHashAndExpires(t *testing.T) {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	store := newMemoryStore()
	service := testService(store, &now)

	grant, err := service.IssueTicket(identity.PublicUser{ID: "user-1", Status: identity.UserStatusEnabled}, "session-1")
	if err != nil {
		t.Fatalf("IssueTicket() error = %v", err)
	}
	if grant.BindingToken != "binding-secret" || grant.ExpiresAt != now.Add(5*time.Minute) {
		t.Fatalf("grant = %+v", grant)
	}
	if store.storedTokenHash == "" || store.storedTokenHash == grant.BindingToken {
		t.Fatalf("stored token hash = %q", store.storedTokenHash)
	}
}

func TestRegisterConsumesTicketOnceAndIssuesHashedCredential(t *testing.T) {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	store := newMemoryStore()
	store.activeSessions["session-1:user-1"] = true
	service := testService(store, &now)
	grant, _ := service.IssueTicket(identity.PublicUser{ID: "user-1", Status: identity.UserStatusEnabled}, "session-1")

	registration, err := service.RegisterLocal(RegisterLocalInput{
		BindingToken:         grant.BindingToken,
		AgentID:              "agent-1",
		DeviceID:             "device-1",
		AgentVersion:         "0.2.0",
		ContractMajorVersion: cloudagent.MajorVersion,
		ContractRevision:     cloudagent.ContractRevision,
	})
	if err != nil {
		t.Fatalf("RegisterLocal() error = %v", err)
	}
	if registration.Node.UserID != "user-1" || registration.Node.SessionID != "session-1" {
		t.Fatalf("node = %+v", registration.Node)
	}
	if registration.NodeCredential != "node-secret" || store.storedCredential == registration.NodeCredential {
		t.Fatalf("credential handling is unsafe: grant=%q stored=%q", registration.NodeCredential, store.storedCredential)
	}
	if _, err := service.RegisterLocal(RegisterLocalInput{BindingToken: grant.BindingToken, AgentID: "agent-2", DeviceID: "device-2", AgentVersion: "0.2.0", ContractMajorVersion: cloudagent.MajorVersion, ContractRevision: cloudagent.ContractRevision}); !errors.Is(err, ErrBindingTicketInvalid) {
		t.Fatalf("second RegisterLocal() error = %v", err)
	}
}

func TestRuntimeReportRequiresActiveBoundSession(t *testing.T) {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	store := newMemoryStore()
	store.activeSessions["session-1:user-1"] = true
	service := testService(store, &now)
	grant, _ := service.IssueTicket(identity.PublicUser{ID: "user-1", Status: identity.UserStatusEnabled}, "session-1")
	registration, _ := service.RegisterLocal(RegisterLocalInput{BindingToken: grant.BindingToken, AgentID: "agent-1", DeviceID: "device-1", AgentVersion: "0.2.0", ContractMajorVersion: cloudagent.MajorVersion, ContractRevision: cloudagent.ContractRevision})

	store.activeSessions["session-1:user-1"] = false
	err := service.ReportRuntime(registration.Node.ID, registration.NodeCredential, validRuntimeReport())
	if !errors.Is(err, ErrBoundSessionInvalid) {
		t.Fatalf("ReportRuntime() error = %v", err)
	}
}

func TestRuntimeReportRejectsCredentialForReplacedNode(t *testing.T) {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	store := newMemoryStore()
	store.nodes[secretHash("old-node-secret")] = AgentNode{ID: "old-node", Mode: "local", Status: AgentStatusReplaced, UserID: "user-1", SessionID: "session-1"}
	service := testService(store, &now)

	err := service.ReportRuntime("old-node", "old-node-secret", validRuntimeReport())
	if !errors.Is(err, ErrNodeCredentialInvalid) {
		t.Fatalf("ReportRuntime() error = %v", err)
	}
}

func TestRuntimeReportValidatesOwnerAndEveryActiveProfile(t *testing.T) {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	store := newMemoryStore()
	store.activeSessions["session-1:user-1"] = true
	store.validProfiles["user-1:profile-1"] = true
	store.validProfiles["user-1:profile-2"] = true
	service := testService(store, &now)
	grant, _ := service.IssueTicket(identity.PublicUser{ID: "user-1", Status: identity.UserStatusEnabled}, "session-1")
	registration, _ := service.RegisterLocal(RegisterLocalInput{BindingToken: grant.BindingToken, AgentID: "agent-1", DeviceID: "device-1", AgentVersion: "0.2.0", ContractMajorVersion: cloudagent.MajorVersion, ContractRevision: cloudagent.ContractRevision})

	report := validRuntimeReport()
	if err := service.ReportRuntime(registration.Node.ID, registration.NodeCredential, report); err != nil {
		t.Fatalf("ReportRuntime() error = %v", err)
	}
	if store.appliedNode.UserID != "user-1" || len(store.appliedReport.BitProfileIDs) != 2 {
		t.Fatalf("applied node/report = %+v / %+v", store.appliedNode, store.appliedReport)
	}

	report.MainUserID = "wrong-main"
	if err := service.ReportRuntime(registration.Node.ID, registration.NodeCredential, report); !errors.Is(err, ErrProfileOwnershipMismatch) {
		t.Fatalf("owner mismatch error = %v", err)
	}
	report.MainUserID = "main-user-1"
	report.BitProfileIDs = append(report.BitProfileIDs, "unknown-profile")
	if err := service.ReportRuntime(registration.Node.ID, registration.NodeCredential, report); !errors.Is(err, ErrProfileOwnershipMismatch) {
		t.Fatalf("profile mismatch error = %v", err)
	}
}

func validRuntimeReport() RuntimeReport {
	return RuntimeReport{
		OperatingSystem:  "macos",
		CPUArchitecture:  "arm64",
		AgentVersion:     "0.2.0",
		PythonVersion:    "3.12.11",
		FFmpeg:           DependencyFact{Status: "normal", Version: "7.1"},
		WorkdirStatus:    "normal",
		Disk:             DiskFact{Status: "normal", FreeMegabytes: 8192},
		BitBrowserStatus: "normal",
		MainUserID:       "main-user-1",
		BitProfileIDs:    []string{"profile-1", "profile-2"},
	}
}
