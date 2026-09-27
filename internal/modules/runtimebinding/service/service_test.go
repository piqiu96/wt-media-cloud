package service

import (
	"errors"
	"fmt"
	"sort"
	"testing"
	"time"

	cloudagentservice "github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/service"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
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

func (s *memoryStore) IsSessionActive(sessionID string, userID identityservice.UserID, at time.Time) (bool, error) {
	return s.activeSessions[runtimeKey(sessionID, userID)], nil
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

func (s *memoryStore) CheckLocalTrust(userID identityservice.UserID, nodeID string, at time.Time, freshness time.Duration) (bool, error) {
	for _, node := range s.nodes {
		if node.ID == nodeID && node.UserID == userID && node.Status == cloudagentservice.AgentStatusOnline &&
			node.LastHeartbeatAt.After(at.Add(-freshness)) && s.activeSessions[runtimeKey(node.SessionID, userID)] {
			return true, nil
		}
	}
	return false, nil
}

// The double has to model the query's *filter and its order*, not just its shape:
// a fake that returned whichever node it iterated first would make a service test
// of "the newest device wins" pass without the ordering being implemented
// anywhere. The SQL itself is pinned separately, by text, in the repository's
// sqlmock tests — the two can still disagree, which is why both exist.
//
// There is deliberately no heartbeat filter here, because there is none in the
// query this stands in for; a double that kept one would make the service tests
// pass while the real resolver refused a bound machine that had not reported
// recently. Its absence is what makes those tests able to fail.
func (s *memoryStore) FindTrustedLocalNode(userID identityservice.UserID) (AgentNode, bool, error) {
	candidates := make([]AgentNode, 0, len(s.nodes))
	for _, node := range s.nodes {
		if node.UserID != userID || node.Mode != "local" || node.Status != cloudagentservice.AgentStatusOnline {
			continue
		}
		if !s.activeSessions[runtimeKey(node.SessionID, userID)] {
			continue
		}
		candidates = append(candidates, node)
	}
	if len(candidates) == 0 {
		return AgentNode{}, false, nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		if !candidates[i].LastHeartbeatAt.Equal(candidates[j].LastHeartbeatAt) {
			return candidates[i].LastHeartbeatAt.After(candidates[j].LastHeartbeatAt)
		}
		return candidates[i].ID < candidates[j].ID
	})
	return candidates[0], true, nil
}

func (s *memoryStore) ValidateRuntimeProfiles(userID identityservice.UserID, mainUserID string, profileIDs []string) (bool, error) {
	if mainUserID != "main-user-1" {
		return false, nil
	}
	if len(profileIDs) == 0 {
		return true, nil
	}
	for _, profileID := range profileIDs {
		if !s.validProfiles[runtimeKey(profileID, userID)] {
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

	grant, err := service.IssueTicket(identityservice.PublicUser{ID: identityservice.UserID(1), Status: identityservice.UserStatusEnabled}, "session-1")
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
	store.activeSessions[runtimeKey("session-1", 1)] = true
	service := testService(store, &now)
	grant, _ := service.IssueTicket(identityservice.PublicUser{ID: identityservice.UserID(1), Status: identityservice.UserStatusEnabled}, "session-1")

	registration, err := service.RegisterLocal(RegisterLocalInput{
		BindingToken:         grant.BindingToken,
		AgentID:              "agent-1",
		DeviceID:             "device-1",
		AgentVersion:         "0.2.0",
		ContractMajorVersion: cloudagentservice.MajorVersion,
		ContractRevision:     cloudagentservice.ContractRevision,
	})
	if err != nil {
		t.Fatalf("RegisterLocal() error = %v", err)
	}
	if registration.Node.UserID != identityservice.UserID(1) || registration.Node.SessionID != "session-1" {
		t.Fatalf("node = %+v", registration.Node)
	}
	if registration.NodeCredential != "node-secret" || store.storedCredential == registration.NodeCredential {
		t.Fatalf("credential handling is unsafe: grant=%q stored=%q", registration.NodeCredential, store.storedCredential)
	}
	if _, err := service.RegisterLocal(RegisterLocalInput{BindingToken: grant.BindingToken, AgentID: "agent-2", DeviceID: "device-2", AgentVersion: "0.2.0", ContractMajorVersion: cloudagentservice.MajorVersion, ContractRevision: cloudagentservice.ContractRevision}); !errors.Is(err, ErrBindingTicketInvalid) {
		t.Fatalf("second RegisterLocal() error = %v", err)
	}
}

func TestRuntimeReportRequiresActiveBoundSession(t *testing.T) {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	store := newMemoryStore()
	store.activeSessions[runtimeKey("session-1", 1)] = true
	service := testService(store, &now)
	grant, _ := service.IssueTicket(identityservice.PublicUser{ID: identityservice.UserID(1), Status: identityservice.UserStatusEnabled}, "session-1")
	registration, _ := service.RegisterLocal(RegisterLocalInput{BindingToken: grant.BindingToken, AgentID: "agent-1", DeviceID: "device-1", AgentVersion: "0.2.0", ContractMajorVersion: cloudagentservice.MajorVersion, ContractRevision: cloudagentservice.ContractRevision})

	store.activeSessions[runtimeKey("session-1", 1)] = false
	err := service.ReportRuntime(registration.Node.ID, registration.NodeCredential, validRuntimeReport())
	if !errors.Is(err, ErrBoundSessionInvalid) {
		t.Fatalf("ReportRuntime() error = %v", err)
	}
}

func TestRuntimeReportRejectsCredentialForReplacedNode(t *testing.T) {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	store := newMemoryStore()
	store.nodes[secretHash("old-node-secret")] = AgentNode{ID: "old-node", Mode: "local", Status: AgentStatusReplaced, UserID: identityservice.UserID(1), SessionID: "session-1"}
	service := testService(store, &now)

	err := service.ReportRuntime("old-node", "old-node-secret", validRuntimeReport())
	if !errors.Is(err, ErrNodeCredentialInvalid) {
		t.Fatalf("ReportRuntime() error = %v", err)
	}
}

func TestCheckLocalTrustRequiresFreshOnlineNodeAndActiveSession(t *testing.T) {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	store := newMemoryStore()
	store.activeSessions[runtimeKey("session-1", 1)] = true
	store.nodes["node-secret"] = AgentNode{ID: "node-1", Mode: "local", Status: cloudagentservice.AgentStatusOnline, UserID: identityservice.UserID(1), SessionID: "session-1", LastHeartbeatAt: now}
	service := testService(store, &now)

	if err := service.CheckLocalTrust(identityservice.UserID(1), "node-1"); err != nil {
		t.Fatalf("CheckLocalTrust() error = %v", err)
	}
	store.activeSessions[runtimeKey("session-1", 1)] = false
	if err := service.CheckLocalTrust(identityservice.UserID(1), "node-1"); !errors.Is(err, ErrLocalTrustUnavailable) {
		t.Fatalf("inactive session error = %v", err)
	}
	store.activeSessions[runtimeKey("session-1", 1)] = true
	now = now.Add(2 * time.Minute)
	if err := service.CheckLocalTrust(identityservice.UserID(1), "node-1"); !errors.Is(err, ErrLocalTrustUnavailable) {
		t.Fatalf("stale node error = %v", err)
	}
}

func TestRuntimeReportValidatesMainIdentityAndLeavesUnknownProfilesToDiff(t *testing.T) {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	store := newMemoryStore()
	store.activeSessions[runtimeKey("session-1", 1)] = true
	store.validProfiles[runtimeKey("profile-1", 1)] = true
	store.validProfiles[runtimeKey("profile-2", 1)] = true
	service := testService(store, &now)
	grant, _ := service.IssueTicket(identityservice.PublicUser{ID: identityservice.UserID(1), Status: identityservice.UserStatusEnabled}, "session-1")
	registration, _ := service.RegisterLocal(RegisterLocalInput{BindingToken: grant.BindingToken, AgentID: "agent-1", DeviceID: "device-1", AgentVersion: "0.2.0", ContractMajorVersion: cloudagentservice.MajorVersion, ContractRevision: cloudagentservice.ContractRevision})

	report := validRuntimeReport()
	if err := service.ReportRuntime(registration.Node.ID, registration.NodeCredential, report); err != nil {
		t.Fatalf("ReportRuntime() error = %v", err)
	}
	if store.appliedNode.UserID != identityservice.UserID(1) || len(store.appliedReport.BitProfileIDs) != 2 {
		t.Fatalf("applied node/report = %+v / %+v", store.appliedNode, store.appliedReport)
	}

	report.MainUserID = "wrong-main"
	if err := service.ReportRuntime(registration.Node.ID, registration.NodeCredential, report); !errors.Is(err, ErrProfileOwnershipMismatch) {
		t.Fatalf("owner mismatch error = %v", err)
	}
	report.MainUserID = "main-user-1"
	report.BitProfileIDs = append(report.BitProfileIDs, "unknown-profile")
	if err := service.ReportRuntime(registration.Node.ID, registration.NodeCredential, report); err != nil {
		t.Fatalf("unknown local profile should be accepted for later diff handling, got %v", err)
	}
	if len(store.appliedReport.BitProfileIDs) != 3 {
		t.Fatalf("applied profile ids = %+v, want 3 ids preserved for runtime projection", store.appliedReport.BitProfileIDs)
	}
}

func liveNode(id, sessionID string, userID identityservice.UserID, heartbeat time.Time) AgentNode {
	return AgentNode{ID: id, Mode: "local", Status: cloudagentservice.AgentStatusOnline, UserID: userID, SessionID: sessionID, LastHeartbeatAt: heartbeat}
}

// The claim route offers no node id, so the credential has to be enough on its
// own. The second arm is what keeps that from becoming "any valid credential
// identifies any node": when a caller *does* name a node, the name is still
// checked.
func TestAuthenticateNodeCredentialIdentifiesANodeWithoutAnID(t *testing.T) {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	store := newMemoryStore()
	store.activeSessions[runtimeKey("session-1", 1)] = true
	store.nodes[secretHash("node-secret")] = liveNode("node-1", "session-1", identityservice.UserID(1), now)
	service := testService(store, &now)

	node, err := service.AuthenticateNodeCredential("node-secret")
	if err != nil {
		t.Fatalf("AuthenticateNodeCredential() error = %v", err)
	}
	if node.ID != "node-1" || node.UserID != identityservice.UserID(1) {
		t.Fatalf("node = %+v", node)
	}
	if _, err := service.AuthenticateNode("node-2", "node-secret"); !errors.Is(err, ErrNodeCredentialInvalid) {
		t.Fatalf("AuthenticateNode() with a wrong id error = %v", err)
	}
	if _, err := service.AuthenticateNodeCredential(""); !errors.Is(err, ErrNodeCredentialInvalid) {
		t.Fatalf("AuthenticateNodeCredential() with no credential error = %v", err)
	}
}

// What the credential route does not relax: a replaced node's credential was
// superseded by a later registration and must not be usable, and the bound session
// still has to be live.
func TestAuthenticateNodeCredentialStillRefusesAReplacedNodeAndADeadSession(t *testing.T) {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	store := newMemoryStore()
	store.activeSessions[runtimeKey("session-1", 1)] = true
	replaced := liveNode("node-1", "session-1", identityservice.UserID(1), now)
	replaced.Status = AgentStatusReplaced
	store.nodes[secretHash("replaced-secret")] = replaced
	store.nodes[secretHash("node-secret")] = liveNode("node-2", "session-1", identityservice.UserID(1), now)
	service := testService(store, &now)

	if _, err := service.AuthenticateNodeCredential("replaced-secret"); !errors.Is(err, ErrNodeCredentialInvalid) {
		t.Fatalf("replaced node error = %v", err)
	}
	store.activeSessions[runtimeKey("session-1", 1)] = false
	if _, err := service.AuthenticateNodeCredential("node-secret"); !errors.Is(err, ErrBoundSessionInvalid) {
		t.Fatalf("dead session error = %v", err)
	}
}

// The session route knows the user but not the device, so the choice among that
// user's bound devices has to be made here. The newest heartbeat wins, because a
// user who has just opened a second machine means to use it.
//
// Both heartbeats are hours old on purpose. Queueing a download must not depend on
// how recently the machine reported — the operator cannot make the Agent report
// without going to another page and pressing a button — so this arm is red for any
// implementation that still bounds the heartbeat, whichever bound it picks.
func TestResolveTrustedLocalNodePicksTheNewestBoundDeviceDespiteStaleHeartbeats(t *testing.T) {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	store := newMemoryStore()
	store.activeSessions[runtimeKey("session-1", 1)] = true
	store.nodes[secretHash("older")] = liveNode("node-a", "session-1", identityservice.UserID(1), now.Add(-3*time.Hour))
	store.nodes[secretHash("newer")] = liveNode("node-b", "session-1", identityservice.UserID(1), now.Add(-2*time.Hour))
	service := testService(store, &now)

	node, err := service.ResolveTrustedLocalNode(identityservice.UserID(1))
	if err != nil {
		t.Fatalf("ResolveTrustedLocalNode() error = %v", err)
	}
	if node.ID != "node-b" {
		t.Fatalf("node = %+v, want the newest heartbeat", node)
	}
}

// "No bound node" and "a node that failed its trust check" are the same answer to
// the caller — there is nothing to download to right now — so they share one error.
// What no longer produces that error is a node that is simply idle: its heartbeat
// is not part of the question, and the other conditions still are.
func TestResolveTrustedLocalNodeReportsNoBoundNode(t *testing.T) {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	store := newMemoryStore()
	store.activeSessions[runtimeKey("session-1", 1)] = true
	service := testService(store, &now)

	if _, err := service.ResolveTrustedLocalNode(identityservice.UserID(1)); !errors.Is(err, ErrLocalTrustUnavailable) {
		t.Fatalf("no node at all error = %v", err)
	}
	// A node whose session is no longer active is not usable however it reported.
	store.nodes[secretHash("stale")] = liveNode("node-a", "session-2", identityservice.UserID(1), now.Add(-2*time.Minute))
	if _, err := service.ResolveTrustedLocalNode(identityservice.UserID(1)); !errors.Is(err, ErrLocalTrustUnavailable) {
		t.Fatalf("inactive session error = %v", err)
	}
	// Another user's live node is not this user's node.
	store.nodes[secretHash("other")] = liveNode("node-b", "session-1", identityservice.UserID(2), now)
	if _, err := service.ResolveTrustedLocalNode(identityservice.UserID(1)); !errors.Is(err, ErrLocalTrustUnavailable) {
		t.Fatalf("another user's node error = %v", err)
	}
	// The first user's own node, on an active session, is found — stale heartbeat
	// and all. This is the arm the old bound made red.
	store.nodes[secretHash("bound")] = liveNode("node-c", "session-1", identityservice.UserID(1), now.Add(-2*time.Hour))
	node, err := service.ResolveTrustedLocalNode(identityservice.UserID(1))
	if err != nil {
		t.Fatalf("stale but bound node error = %v", err)
	}
	if node.ID != "node-c" {
		t.Fatalf("node = %+v, want the user's own bound node", node)
	}
}

func runtimeKey(value string, userID identityservice.UserID) string {
	return fmt.Sprintf("%s:%d", value, userID)
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
