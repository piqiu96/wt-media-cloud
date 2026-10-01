// Package runtimebinding owns local Agent session binding and runtime presence.
package service

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"time"

	cloudagentservice "github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/service"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding/model"
	"github.com/wt-media/wt-media-cloud/internal/shared/id"
)

var (
	ErrForbidden                = errors.New("runtime binding operation is forbidden")
	ErrInvalidInput             = errors.New("runtime binding input is invalid")
	ErrBindingTicketInvalid     = errors.New("binding ticket is invalid or expired")
	ErrBoundSessionInvalid      = errors.New("bound session is no longer active")
	ErrNodeCredentialInvalid    = errors.New("node credential is invalid")
	ErrLocalTrustUnavailable    = errors.New("local runtime trust is unavailable")
	ErrProfileOwnershipMismatch = errors.New("runtime profiles do not match confirmed ownership")
	ErrDeviceNotBound           = model.ErrDeviceNotBound
	ErrDeviceMismatch           = model.ErrDeviceMismatch
)

const AgentStatusReplaced = "replaced"

type (
	BindingTicket      = model.BindingTicket
	BindingTicketGrant = dto.BindingTicketGrant
	AgentNode          = model.AgentNode
	RegisterLocalInput = dto.RegisterLocalInput
	Registration       = dto.Registration
	DependencyFact     = model.DependencyFact
	DiskFact           = model.DiskFact
	RuntimeReport      = dto.RuntimeReport
	DeviceBinding      = model.DeviceBinding
)

type Store interface {
	CreateTicket(BindingTicket) error
	ConsumeTicket(tokenHash string, at time.Time) (BindingTicket, bool, error)
	IsSessionActive(sessionID string, userID identityservice.UserID, at time.Time) (bool, error)
	SaveNode(AgentNode) error
	GetDeviceBinding(identityservice.UserID) (DeviceBinding, error)
	UnbindDevice(identityservice.UserID, time.Time) error
	IsDeviceBound(identityservice.UserID, string) (bool, error)
	FindNodeByCredentialHash(hash string) (AgentNode, bool, error)
	CheckLocalTrust(userID identityservice.UserID, nodeID string, at time.Time, freshness time.Duration) (bool, error)
	FindTrustedLocalNode(userID identityservice.UserID) (AgentNode, bool, error)
	ValidateRuntimeProfiles(userID identityservice.UserID, mainUserID string, profileIDs []string) (bool, error)
	ApplyRuntimeReport(node AgentNode, report RuntimeReport, at time.Time) error
}

type Service struct {
	store     Store
	now       func() time.Time
	newID     func(string) string
	newSecret func() string
	ticketTTL time.Duration
	freshness time.Duration
}

type Option func(*Service)

func WithClock(now func() time.Time) Option            { return func(s *Service) { s.now = now } }
func WithIDGenerator(newID func(string) string) Option { return func(s *Service) { s.newID = newID } }
func WithSecretGenerator(newSecret func() string) Option {
	return func(s *Service) { s.newSecret = newSecret }
}
func WithTicketTTL(ttl time.Duration) Option { return func(s *Service) { s.ticketTTL = ttl } }
func WithFreshness(freshness time.Duration) Option {
	return func(s *Service) { s.freshness = freshness }
}

func newService(store Store, options ...Option) *Service {
	service := &Service{
		store:     store,
		now:       func() time.Time { return time.Now().UTC() },
		newID:     id.NewID,
		newSecret: randomSecret,
		ticketTTL: 5 * time.Minute,
		freshness: 90 * time.Second,
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func (s *Service) IssueTicket(actor identityservice.PublicUser, sessionID string) (BindingTicketGrant, error) {
	if actor.ID <= 0 || actor.Status != identityservice.UserStatusEnabled || strings.TrimSpace(sessionID) == "" {
		return BindingTicketGrant{}, ErrForbidden
	}
	now := s.now()
	token := s.newSecret()
	if token == "" {
		return BindingTicketGrant{}, ErrInvalidInput
	}
	ticket := BindingTicket{
		ID: s.newID("agent-ticket"), UserID: actor.ID, SessionID: sessionID,
		TokenHash: secretHash(token), CreatedAt: now, ExpiresAt: now.Add(s.ticketTTL),
	}
	if err := s.store.CreateTicket(ticket); err != nil {
		return BindingTicketGrant{}, err
	}
	return BindingTicketGrant{TicketID: ticket.ID, BindingToken: token, ExpiresAt: ticket.ExpiresAt}, nil
}

func (s *Service) RegisterLocal(input RegisterLocalInput) (Registration, error) {
	if strings.TrimSpace(input.BindingToken) == "" || strings.TrimSpace(input.AgentID) == "" ||
		strings.TrimSpace(input.DeviceID) == "" || strings.TrimSpace(input.AgentVersion) == "" {
		return Registration{}, ErrInvalidInput
	}
	if !cloudagentservice.IsAgentCompatible(input.ContractMajorVersion, input.ContractRevision) {
		return Registration{}, cloudagentservice.ErrIncompatibleAgent
	}
	publicKey, err := base64.RawURLEncoding.DecodeString(input.DevicePublicKey)
	signature, signatureErr := base64.RawURLEncoding.DecodeString(input.DeviceSignature)
	fingerprint := sha256.Sum256(publicKey)
	if err != nil || signatureErr != nil || len(publicKey) != ed25519.PublicKeySize || len(signature) != ed25519.SignatureSize ||
		len(input.DeviceName) > 64 || hex.EncodeToString(fingerprint[:]) != input.DeviceID {
		return Registration{}, ErrInvalidInput
	}
	if !ed25519.Verify(ed25519.PublicKey(publicKey), deviceProofMessage(input.BindingToken, input.DeviceID), signature) {
		return Registration{}, ErrForbidden
	}
	now := s.now()
	ticket, ok, err := s.store.ConsumeTicket(secretHash(input.BindingToken), now)
	if err != nil {
		return Registration{}, err
	}
	if !ok {
		return Registration{}, ErrBindingTicketInvalid
	}
	active, err := s.store.IsSessionActive(ticket.SessionID, ticket.UserID, now)
	if err != nil {
		return Registration{}, err
	}
	if !active {
		return Registration{}, ErrBoundSessionInvalid
	}
	credential := s.newSecret()
	node := AgentNode{
		ID: s.newID("agent-node"), AgentID: strings.TrimSpace(input.AgentID), DeviceID: strings.TrimSpace(input.DeviceID),
		DevicePublicKey: publicKey, DeviceName: strings.TrimSpace(input.DeviceName), BindDevice: input.BindDevice,
		UserID: ticket.UserID, SessionID: ticket.SessionID, Mode: "local", AgentVersion: strings.TrimSpace(input.AgentVersion),
		ContractMajorVersion: input.ContractMajorVersion, ContractRevision: input.ContractRevision,
		Status: cloudagentservice.AgentStatusOnline, CredentialHash: secretHash(credential), RegisteredAt: now, LastHeartbeatAt: now,
	}
	if err := s.store.SaveNode(node); err != nil {
		return Registration{}, err
	}
	return Registration{Node: node, NodeCredential: credential}, nil
}

func deviceProofMessage(token, deviceID string) []byte {
	return []byte("wt-media-device-v1\n" + token + "\n" + deviceID)
}

func (s *Service) GetDeviceBinding(userID identityservice.UserID) (DeviceBinding, error) {
	if userID <= 0 {
		return DeviceBinding{}, ErrForbidden
	}
	return s.store.GetDeviceBinding(userID)
}

func (s *Service) UnbindDevice(userID identityservice.UserID) error {
	if userID <= 0 {
		return ErrForbidden
	}
	return s.store.UnbindDevice(userID, s.now())
}

func (s *Service) CheckLocalTrust(userID identityservice.UserID, nodeID string) error {
	if userID <= 0 || strings.TrimSpace(nodeID) == "" {
		return ErrInvalidInput
	}
	trusted, err := s.store.CheckLocalTrust(userID, strings.TrimSpace(nodeID), s.now(), s.freshness)
	if err != nil {
		return err
	}
	if !trusted {
		return ErrLocalTrustUnavailable
	}
	return nil
}

func (s *Service) ReportRuntime(nodeID, credential string, report RuntimeReport) error {
	if strings.TrimSpace(nodeID) == "" || strings.TrimSpace(credential) == "" || !validReport(report) {
		return ErrInvalidInput
	}
	node, err := s.AuthenticateNode(nodeID, credential)
	if err != nil {
		return err
	}
	now := s.now()
	if report.BitBrowserStatus == "normal" {
		profileIDs := uniqueSorted(report.BitProfileIDs)
		if len(profileIDs) != len(report.BitProfileIDs) {
			return ErrInvalidInput
		}
		valid, err := s.store.ValidateRuntimeProfiles(node.UserID, report.MainUserID, nil)
		if err != nil {
			return err
		}
		if !valid {
			return ErrProfileOwnershipMismatch
		}
		report.BitProfileIDs = profileIDs
	}
	node.LastHeartbeatAt = now
	node.Status = cloudagentservice.AgentStatusOnline
	return s.store.ApplyRuntimeReport(node, report, now)
}

// AuthenticateNode verifies the bearer node credential and the bound active
// user session for other Cloud security modules. It never exposes the hash.
func (s *Service) AuthenticateNode(nodeID, credential string) (AgentNode, error) {
	if strings.TrimSpace(nodeID) == "" {
		return AgentNode{}, ErrNodeCredentialInvalid
	}
	node, err := s.authenticateCredential(credential)
	if err != nil {
		return AgentNode{}, err
	}
	if node.ID != nodeID {
		return AgentNode{}, ErrNodeCredentialInvalid
	}
	return node, nil
}

// AuthenticateNodeCredential identifies a node by its credential alone.
//
// It exists because a route can offer nothing else. The frozen
// `POST /api/v1/cloud-agent/file-transfer-tasks/claim` has no path parameter and
// no request body — a polling executor has no task to name and nothing to send —
// so the node id a caller would otherwise pass is not available to be checked,
// and `node.ID == nodeID` would have nothing to compare against. The credential
// itself is what the bearer token carries, and it is the only thing that has to be
// verified.
//
// What is *not* relaxed: the credential's hash is the lookup key (so a caller
// cannot name a node it has no secret for), and the bound session still has to be
// active. A replaced node is still refused, because its credential was superseded
// by a later registration.
func (s *Service) AuthenticateNodeCredential(credential string) (AgentNode, error) {
	return s.authenticateCredential(credential)
}

func (s *Service) authenticateCredential(credential string) (AgentNode, error) {
	if strings.TrimSpace(credential) == "" {
		return AgentNode{}, ErrNodeCredentialInvalid
	}
	node, ok, err := s.store.FindNodeByCredentialHash(secretHash(credential))
	if err != nil {
		return AgentNode{}, err
	}
	if !ok || node.Mode != "local" || node.Status == AgentStatusReplaced {
		return AgentNode{}, ErrNodeCredentialInvalid
	}
	bound, err := s.store.IsDeviceBound(node.UserID, node.DeviceID)
	if err != nil {
		return AgentNode{}, err
	}
	if !bound {
		return AgentNode{}, ErrDeviceMismatch
	}
	active, err := s.store.IsSessionActive(node.SessionID, node.UserID, s.now())
	if err != nil {
		return AgentNode{}, err
	}
	if !active {
		return AgentNode{}, ErrBoundSessionInvalid
	}
	return node, nil
}

// ResolveTrustedLocalNode answers "which of this user's devices should receive a
// download" without the caller naming one, and reports
// `ErrLocalTrustUnavailable` when there is none — the same error a named node's
// failed trust check produces, because from the caller's side the outcome is
// identical: there is no bound node for this user right now.
//
// Note what it does *not* ask: how recently that node reported. Queueing a
// download is not the same act as driving the operator's browser from this
// process, and only the latter needs a heartbeat the operator just produced by
// hand. See `repository.FindTrustedLocalNode` for the full argument.
func (s *Service) ResolveTrustedLocalNode(userID identityservice.UserID) (AgentNode, error) {
	if userID <= 0 {
		return AgentNode{}, ErrInvalidInput
	}
	node, found, err := s.store.FindTrustedLocalNode(userID)
	if err != nil {
		return AgentNode{}, err
	}
	if !found {
		return AgentNode{}, ErrLocalTrustUnavailable
	}
	return node, nil
}

func validReport(report RuntimeReport) bool {
	if !in(report.OperatingSystem, "macos", "windows", "linux", "unsupported") ||
		!in(report.CPUArchitecture, "x86_64", "arm64", "unsupported") ||
		strings.TrimSpace(report.AgentVersion) == "" || strings.TrimSpace(report.PythonVersion) == "" ||
		!validDependency(report.FFmpeg) || !in(report.WorkdirStatus, "normal", "abnormal", "not_installed", "needs_upgrade", "unreachable", "user_action_required") ||
		!in(report.Disk.Status, "normal", "abnormal", "user_action_required") || report.Disk.FreeMegabytes < 0 ||
		!in(report.BitBrowserStatus, "normal", "unreachable", "identity_unverifiable", "not_installed", "needs_upgrade", "user_action_required") {
		return false
	}
	if report.BitBrowserStatus == "normal" {
		return strings.TrimSpace(report.MainUserID) != "" && allNonEmpty(report.BitProfileIDs)
	}
	return report.MainUserID == "" && len(report.BitProfileIDs) == 0
}

func validDependency(value DependencyFact) bool {
	return in(value.Status, "normal", "abnormal", "not_installed", "needs_upgrade", "unreachable", "user_action_required") &&
		(value.Status != "normal" || strings.TrimSpace(value.Version) != "")
}

func in(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}

func allNonEmpty(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return false
		}
	}
	return true
}

func uniqueSorted(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	for index := 1; index < len(result); index++ {
		if result[index] == result[index-1] {
			return nil
		}
	}
	return result
}

func secretHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func randomSecret() string {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(value)
}
