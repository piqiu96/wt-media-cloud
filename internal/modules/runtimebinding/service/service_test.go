package service

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
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
	boundDevices     map[identityservice.UserID]string
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		tickets:        make(map[string]BindingTicket),
		nodes:          make(map[string]AgentNode),
		activeSessions: make(map[string]bool),
		validProfiles:  make(map[string]bool),
		boundDevices:   make(map[identityservice.UserID]string),
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
	if bound := s.boundDevices[node.UserID]; bound != "" && bound != node.DeviceID {
		return ErrDeviceMismatch
	}
	if node.BindDevice {
		s.boundDevices[node.UserID] = node.DeviceID
	}
	// A re-registration supersedes the previous node for the same device, exactly
	// as the repository's SQL marks it: the credential a replacement just took
	// over has to stop authenticating, or the renewal test would pass while the
	// real flow kept two live credentials for one device.
	for key, existing := range s.nodes {
		if existing.UserID == node.UserID && existing.DeviceID == node.DeviceID && existing.Status != AgentStatusReplaced {
			existing.Status = AgentStatusReplaced
			s.nodes[key] = existing
		}
	}
	s.nodes[node.CredentialHash] = node
	s.storedCredential = node.CredentialHash
	return nil
}

func (s *memoryStore) GetDeviceBinding(userID identityservice.UserID) (DeviceBinding, error) {
	id := s.boundDevices[userID]
	return DeviceBinding{Bound: id != "", DeviceID: id}, nil
}
func (s *memoryStore) UnbindDevice(userID identityservice.UserID, _ time.Time) error {
	if s.boundDevices[userID] == "" {
		return ErrDeviceNotBound
	}
	delete(s.boundDevices, userID)
	for key, node := range s.nodes {
		if node.UserID == userID {
			node.Status = AgentStatusReplaced
			s.nodes[key] = node
		}
	}
	return nil
}
func (s *memoryStore) IsDeviceBound(userID identityservice.UserID, deviceID string) (bool, error) {
	bound, explicit := s.boundDevices[userID]
	if !explicit {
		return true, nil
	} // Existing fixture nodes predate device enrollment.
	return bound == deviceID, nil
}

func signedInput(token, agentID string, bind bool) RegisterLocalInput {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	fingerprint := sha256.Sum256(publicKey)
	deviceID := hex.EncodeToString(fingerprint[:])
	return RegisterLocalInput{
		BindingToken: token, AgentID: agentID, DeviceID: deviceID,
		DevicePublicKey: base64.RawURLEncoding.EncodeToString(publicKey),
		DeviceSignature: base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, deviceProofMessage(token, deviceID))),
		BindDevice:      bind, DeviceName: "测试设备", AgentVersion: "0.2.0",
		ContractMajorVersion: cloudagentservice.MajorVersion, ContractRevision: cloudagentservice.ContractRevision,
	}
}

// A keypair that can sign several tickets with the same device identity — the
// shape a real Desktop has across restarts. `signedInput` above generates a
// fresh key every call, which is right for one-shot registrations and wrong for
// the renewal tests, whose whole point is that the *same* key re-registers.
type deviceSigner struct {
	privateKey ed25519.PrivateKey
	deviceID   string
}

func newDeviceSigner() *deviceSigner {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	fingerprint := sha256.Sum256(publicKey)
	return &deviceSigner{privateKey: privateKey, deviceID: hex.EncodeToString(fingerprint[:])}
}

func (d *deviceSigner) input(token, agentID string, bind bool) RegisterLocalInput {
	publicKey := d.privateKey.Public().(ed25519.PublicKey)
	return RegisterLocalInput{
		BindingToken: token, AgentID: agentID, DeviceID: d.deviceID,
		DevicePublicKey: base64.RawURLEncoding.EncodeToString(publicKey),
		DeviceSignature: base64.RawURLEncoding.EncodeToString(ed25519.Sign(d.privateKey, deviceProofMessage(token, d.deviceID))),
		BindDevice:      bind, DeviceName: "测试设备", AgentVersion: "0.2.0",
		ContractMajorVersion: cloudagentservice.MajorVersion, ContractRevision: cloudagentservice.ContractRevision,
	}
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

// A service whose secret generator never runs dry, for flows that register more
// than once (renewal, unbind→rebind, forged-signature refusal). `testService`
// pins two exact secrets and is the one the one-shot tests assert against.
func serviceWithSecrets(store *memoryStore, now *time.Time) *Service {
	ids := 0
	issued := 0
	return NewService(
		store,
		WithClock(func() time.Time { return *now }),
		WithIDGenerator(func(prefix string) string { ids++; return prefix + "-" + string(rune('0'+ids)) }),
		WithSecretGenerator(func() string {
			issued++
			if issued%2 == 1 {
				return fmt.Sprintf("ticket-secret-%d", issued)
			}
			return fmt.Sprintf("node-secret-%d", issued)
		}),
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

	registration, err := service.RegisterLocal(signedInput(grant.BindingToken, "agent-1", true))
	if err != nil {
		t.Fatalf("RegisterLocal() error = %v", err)
	}
	if registration.Node.UserID != identityservice.UserID(1) || registration.Node.SessionID != "session-1" {
		t.Fatalf("node = %+v", registration.Node)
	}
	if registration.NodeCredential != "node-secret" || store.storedCredential == registration.NodeCredential {
		t.Fatalf("credential handling is unsafe: grant=%q stored=%q", registration.NodeCredential, store.storedCredential)
	}
	if _, err := service.RegisterLocal(signedInput(grant.BindingToken, "agent-2", true)); !errors.Is(err, ErrBindingTicketInvalid) {
		t.Fatalf("second RegisterLocal() error = %v", err)
	}
}

func TestRuntimeReportRequiresActiveBoundSession(t *testing.T) {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	store := newMemoryStore()
	store.activeSessions[runtimeKey("session-1", 1)] = true
	service := testService(store, &now)
	grant, _ := service.IssueTicket(identityservice.PublicUser{ID: identityservice.UserID(1), Status: identityservice.UserStatusEnabled}, "session-1")
	registration, _ := service.RegisterLocal(signedInput(grant.BindingToken, "agent-1", true))

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
	registration, _ := service.RegisterLocal(signedInput(grant.BindingToken, "agent-1", true))

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

// ---- Device binding scenarios (CHG-072) ----

// The same installation re-registers across sessions and restarts with the same
// key. Cloud's `saveNode` refreshes the binding when the key matches and
// supersedes the previous node; the operator is not asked to bind again.
func TestDeviceRenewalReusesTheSameKeyAcrossSessions(t *testing.T) {
	now := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	store := newMemoryStore()
	store.activeSessions[runtimeKey("session-1", 1)] = true
	store.activeSessions[runtimeKey("session-2", 1)] = true
	service := serviceWithSecrets(store, &now)
	signer := newDeviceSigner()
	user := identityservice.PublicUser{ID: identityservice.UserID(1), Status: identityservice.UserStatusEnabled}

	firstGrant, err := service.IssueTicket(user, "session-1")
	if err != nil {
		t.Fatalf("IssueTicket() error = %v", err)
	}
	first, err := service.RegisterLocal(signer.input(firstGrant.BindingToken, "agent-1", true))
	if err != nil {
		t.Fatalf("first RegisterLocal() error = %v", err)
	}
	// The old session ends; the same device signs in again.
	store.activeSessions[runtimeKey("session-1", 1)] = false
	secondGrant, err := service.IssueTicket(user, "session-2")
	if err != nil {
		t.Fatalf("IssueTicket() for the second session error = %v", err)
	}
	renewed, err := service.RegisterLocal(signer.input(secondGrant.BindingToken, "agent-2", true))
	if err != nil {
		t.Fatalf("renewal RegisterLocal() error = %v", err)
	}
	if renewed.Node.SessionID != "session-2" {
		t.Fatalf("renewed node session = %s, want session-2", renewed.Node.SessionID)
	}
	binding, err := service.GetDeviceBinding(identityservice.UserID(1))
	if err != nil || !binding.Bound || binding.DeviceID != signer.deviceID {
		t.Fatalf("binding after renewal = %+v (err %v)", binding, err)
	}
	// The renewal superseded the first node's credential.
	if _, err := service.AuthenticateNodeCredential(first.NodeCredential); !errors.Is(err, ErrNodeCredentialInvalid) {
		t.Fatalf("superseded credential error = %v", err)
	}
	if _, err := service.AuthenticateNodeCredential(renewed.NodeCredential); err != nil {
		t.Fatalf("renewed credential error = %v", err)
	}
}

// A second Desktop with a different key cannot take over the binding: the store
// refuses the mismatched device id, with or without `bind_device`, until the
// bound device is explicitly unbound.
func TestDifferentDeviceKeyIsRefusedUntilUnbind(t *testing.T) {
	now := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	store := newMemoryStore()
	store.activeSessions[runtimeKey("session-1", 1)] = true
	store.activeSessions[runtimeKey("session-2", 1)] = true
	service := serviceWithSecrets(store, &now)
	user := identityservice.PublicUser{ID: identityservice.UserID(1), Status: identityservice.UserStatusEnabled}
	deviceA := newDeviceSigner()
	deviceB := newDeviceSigner()
	if deviceA.deviceID == deviceB.deviceID {
		t.Fatal("two generated devices collided")
	}

	firstGrant, err := service.IssueTicket(user, "session-1")
	if err != nil {
		t.Fatalf("IssueTicket() error = %v", err)
	}
	if _, err := service.RegisterLocal(deviceA.input(firstGrant.BindingToken, "agent-a", true)); err != nil {
		t.Fatalf("bind device A error = %v", err)
	}
	// The same user, a different machine, asking to bind.
	secondGrant, _ := service.IssueTicket(user, "session-2")
	if _, err := service.RegisterLocal(deviceB.input(secondGrant.BindingToken, "agent-b", true)); !errors.Is(err, ErrDeviceMismatch) {
		t.Fatalf("different device with bind error = %v", err)
	}
	// Without `bind_device` the mismatched device is refused just the same.
	thirdGrant, _ := service.IssueTicket(user, "session-2")
	if _, err := service.RegisterLocal(deviceB.input(thirdGrant.BindingToken, "agent-b", false)); !errors.Is(err, ErrDeviceMismatch) {
		t.Fatalf("different device without bind error = %v", err)
	}
	// The original binding survives the refused takeover attempts.
	binding, _ := service.GetDeviceBinding(identityservice.UserID(1))
	if !binding.Bound || binding.DeviceID != deviceA.deviceID {
		t.Fatalf("binding after refused takeover = %+v", binding)
	}
}

// Explicit unbind is the only way to move the binding to a new device. After it
// the old credential is dead and a fresh device binds cleanly.
func TestUnbindThenRebindWithANewDevice(t *testing.T) {
	now := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	store := newMemoryStore()
	store.activeSessions[runtimeKey("session-1", 1)] = true
	store.activeSessions[runtimeKey("session-2", 1)] = true
	service := serviceWithSecrets(store, &now)
	user := identityservice.PublicUser{ID: identityservice.UserID(1), Status: identityservice.UserStatusEnabled}
	deviceA := newDeviceSigner()
	deviceB := newDeviceSigner()

	firstGrant, _ := service.IssueTicket(user, "session-1")
	regA, err := service.RegisterLocal(deviceA.input(firstGrant.BindingToken, "agent-a", true))
	if err != nil {
		t.Fatalf("bind device A error = %v", err)
	}
	if err := service.UnbindDevice(identityservice.UserID(1)); err != nil {
		t.Fatalf("UnbindDevice() error = %v", err)
	}
	binding, _ := service.GetDeviceBinding(identityservice.UserID(1))
	if binding.Bound {
		t.Fatalf("still bound after unbind: %+v", binding)
	}
	if _, err := service.AuthenticateNodeCredential(regA.NodeCredential); !errors.Is(err, ErrNodeCredentialInvalid) {
		t.Fatalf("old credential after unbind error = %v", err)
	}

	secondGrant, _ := service.IssueTicket(user, "session-2")
	regB, err := service.RegisterLocal(deviceB.input(secondGrant.BindingToken, "agent-b", true))
	if err != nil {
		t.Fatalf("rebind with device B error = %v", err)
	}
	binding, _ = service.GetDeviceBinding(identityservice.UserID(1))
	if !binding.Bound || binding.DeviceID != deviceB.deviceID {
		t.Fatalf("binding after rebind = %+v", binding)
	}
	if _, err := service.AuthenticateNodeCredential(regB.NodeCredential); err != nil {
		t.Fatalf("device B credential after rebind error = %v", err)
	}
}

// The registration only accepts a signature proving the key owns the claimed
// device id and was presented with the ticket it signs. Forged or misattributed
// signatures are refused, and the refusals do not consume the one-use ticket.
func TestRegisterRejectsForgedOrMisattributedSignatures(t *testing.T) {
	now := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	store := newMemoryStore()
	store.activeSessions[runtimeKey("session-1", 1)] = true
	service := serviceWithSecrets(store, &now)
	user := identityservice.PublicUser{ID: identityservice.UserID(1), Status: identityservice.UserStatusEnabled}
	signer := newDeviceSigner()

	grant, err := service.IssueTicket(user, "session-1")
	if err != nil {
		t.Fatalf("IssueTicket() error = %v", err)
	}
	good := signer.input(grant.BindingToken, "agent-1", true)

	// A signature over a different device id than the one the key claims is not
	// proof of ownership of the claimed device.
	wrongDevice := good
	wrongDevice.DeviceSignature = base64.RawURLEncoding.EncodeToString(
		ed25519.Sign(signer.privateKey, deviceProofMessage(grant.BindingToken, "not-my-device")),
	)
	if _, err := service.RegisterLocal(wrongDevice); !errors.Is(err, ErrForbidden) {
		t.Fatalf("misattributed signature error = %v", err)
	}

	// A malformed signature (wrong length) is rejected up front.
	badSignature := good
	badSignature.DeviceSignature = base64.RawURLEncoding.EncodeToString([]byte("not-a-signature"))
	if _, err := service.RegisterLocal(badSignature); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("malformed signature error = %v", err)
	}

	// A device id that is not the key's fingerprint is rejected up front.
	badDevice := good
	badDevice.DeviceID = strings.Repeat("0", 64)
	if _, err := service.RegisterLocal(badDevice); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("misattributed device id error = %v", err)
	}

	// The real signature still registers: the refusals above never consumed the ticket.
	if _, err := service.RegisterLocal(good); err != nil {
		t.Fatalf("valid registration after refusals error = %v", err)
	}
}

// A runtime report describing a broken local environment never unbinds the
// device. Only the explicit unbind route does; the binding and the credential
// survive every report the schema accepts.
func TestDegradedDependencyReportDoesNotUnbind(t *testing.T) {
	now := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	store := newMemoryStore()
	store.activeSessions[runtimeKey("session-1", 1)] = true
	service := serviceWithSecrets(store, &now)
	user := identityservice.PublicUser{ID: identityservice.UserID(1), Status: identityservice.UserStatusEnabled}
	signer := newDeviceSigner()

	grant, _ := service.IssueTicket(user, "session-1")
	reg, err := service.RegisterLocal(signer.input(grant.BindingToken, "agent-1", true))
	if err != nil {
		t.Fatalf("RegisterLocal() error = %v", err)
	}

	report := RuntimeReport{
		OperatingSystem:  "macos",
		CPUArchitecture:  "arm64",
		AgentVersion:     "0.2.0",
		PythonVersion:    "3.12.11",
		FFmpeg:           DependencyFact{Status: "abnormal"},
		WorkdirStatus:    "user_action_required",
		Disk:             DiskFact{Status: "user_action_required", FreeMegabytes: 96},
		BitBrowserStatus: "unreachable",
	}
	if err := service.ReportRuntime(reg.Node.ID, reg.NodeCredential, report); err != nil {
		t.Fatalf("degraded report error = %v", err)
	}
	binding, err := service.GetDeviceBinding(identityservice.UserID(1))
	if err != nil || !binding.Bound || binding.DeviceID != signer.deviceID {
		t.Fatalf("device unbound by a degraded report: %+v (err %v)", binding, err)
	}
	if _, err := service.AuthenticateNodeCredential(reg.NodeCredential); err != nil {
		t.Fatalf("credential after degraded report error = %v", err)
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
