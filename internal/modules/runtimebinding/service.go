// Package runtimebinding owns local Agent session binding and runtime presence.
package runtimebinding

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/common"
	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
)

var (
	ErrForbidden                = errors.New("runtime binding operation is forbidden")
	ErrInvalidInput             = errors.New("runtime binding input is invalid")
	ErrBindingTicketInvalid     = errors.New("binding ticket is invalid or expired")
	ErrBoundSessionInvalid      = errors.New("bound session is no longer active")
	ErrNodeCredentialInvalid    = errors.New("node credential is invalid")
	ErrLocalTrustUnavailable    = errors.New("local runtime trust is unavailable")
	ErrProfileOwnershipMismatch = errors.New("runtime profiles do not match confirmed ownership")
)

const AgentStatusReplaced = "replaced"

type BindingTicket struct {
	ID        string
	UserID    identity.UserID
	SessionID string
	TokenHash string
	CreatedAt time.Time
	ExpiresAt time.Time
	UsedAt    *time.Time
}

type BindingTicketGrant struct {
	TicketID     string    `json:"ticket_id"`
	BindingToken string    `json:"binding_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type AgentNode struct {
	ID                   string          `json:"id"`
	AgentID              string          `json:"agent_id"`
	DeviceID             string          `json:"device_id"`
	UserID               identity.UserID `json:"user_id"`
	SessionID            string          `json:"-"`
	Mode                 string          `json:"mode"`
	AgentVersion         string          `json:"agent_version"`
	ContractMajorVersion string          `json:"contract_major_version"`
	ContractRevision     string          `json:"contract_revision"`
	Status               string          `json:"status"`
	CredentialHash       string          `json:"-"`
	RegisteredAt         time.Time       `json:"registered_at"`
	LastHeartbeatAt      time.Time       `json:"last_heartbeat_at"`
}

type RegisterLocalInput struct {
	BindingToken         string `json:"binding_token"`
	AgentID              string `json:"agent_id"`
	DeviceID             string `json:"device_id"`
	AgentVersion         string `json:"agent_version"`
	ContractMajorVersion string `json:"contract_major_version"`
	ContractRevision     string `json:"contract_revision"`
}

type Registration struct {
	Node           AgentNode `json:"node"`
	NodeCredential string    `json:"node_credential"`
}

type DependencyFact struct {
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
}

type DiskFact struct {
	Status        string `json:"status"`
	FreeMegabytes int64  `json:"free_megabytes"`
}

type RuntimeReport struct {
	OperatingSystem  string         `json:"operating_system"`
	CPUArchitecture  string         `json:"cpu_architecture"`
	AgentVersion     string         `json:"agent_version"`
	PythonVersion    string         `json:"python_version"`
	FFmpeg           DependencyFact `json:"ffmpeg"`
	WorkdirStatus    string         `json:"workdir_status"`
	Disk             DiskFact       `json:"disk"`
	BitBrowserStatus string         `json:"bitbrowser_status"`
	MainUserID       string         `json:"main_user_id,omitempty"`
	BitProfileIDs    []string       `json:"bit_profile_ids,omitempty"`
}

type Store interface {
	CreateTicket(BindingTicket) error
	ConsumeTicket(tokenHash string, at time.Time) (BindingTicket, bool, error)
	IsSessionActive(sessionID string, userID identity.UserID, at time.Time) (bool, error)
	SaveNode(AgentNode) error
	FindNodeByCredentialHash(hash string) (AgentNode, bool, error)
	CheckLocalTrust(userID identity.UserID, nodeID string, at time.Time, freshness time.Duration) (bool, error)
	ValidateRuntimeProfiles(userID identity.UserID, mainUserID string, profileIDs []string) (bool, error)
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

func NewService(store Store, options ...Option) *Service {
	service := &Service{
		store:     store,
		now:       func() time.Time { return time.Now().UTC() },
		newID:     common.NewID,
		newSecret: randomSecret,
		ticketTTL: 5 * time.Minute,
		freshness: 90 * time.Second,
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func (s *Service) IssueTicket(actor identity.PublicUser, sessionID string) (BindingTicketGrant, error) {
	if actor.ID <= 0 || actor.Status != identity.UserStatusEnabled || strings.TrimSpace(sessionID) == "" {
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
	if !cloudagent.IsAgentCompatible(input.ContractMajorVersion, input.ContractRevision) {
		return Registration{}, cloudagent.ErrIncompatibleAgent
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
		UserID: ticket.UserID, SessionID: ticket.SessionID, Mode: "local", AgentVersion: strings.TrimSpace(input.AgentVersion),
		ContractMajorVersion: input.ContractMajorVersion, ContractRevision: input.ContractRevision,
		Status: cloudagent.AgentStatusOnline, CredentialHash: secretHash(credential), RegisteredAt: now, LastHeartbeatAt: now,
	}
	if err := s.store.SaveNode(node); err != nil {
		return Registration{}, err
	}
	return Registration{Node: node, NodeCredential: credential}, nil
}

func (s *Service) CheckLocalTrust(userID identity.UserID, nodeID string) error {
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
		valid, err := s.store.ValidateRuntimeProfiles(node.UserID, report.MainUserID, profileIDs)
		if err != nil {
			return err
		}
		if !valid {
			return ErrProfileOwnershipMismatch
		}
		report.BitProfileIDs = profileIDs
	}
	node.LastHeartbeatAt = now
	node.Status = cloudagent.AgentStatusOnline
	return s.store.ApplyRuntimeReport(node, report, now)
}

// AuthenticateNode verifies the bearer node credential and the bound active
// user session for other Cloud security modules. It never exposes the hash.
func (s *Service) AuthenticateNode(nodeID, credential string) (AgentNode, error) {
	if strings.TrimSpace(nodeID) == "" || strings.TrimSpace(credential) == "" {
		return AgentNode{}, ErrNodeCredentialInvalid
	}
	node, ok, err := s.store.FindNodeByCredentialHash(secretHash(credential))
	if err != nil {
		return AgentNode{}, err
	}
	if !ok || node.ID != nodeID || node.Mode != "local" || node.Status == AgentStatusReplaced {
		return AgentNode{}, ErrNodeCredentialInvalid
	}
	now := s.now()
	active, err := s.store.IsSessionActive(node.SessionID, node.UserID, now)
	if err != nil {
		return AgentNode{}, err
	}
	if !active {
		return AgentNode{}, ErrBoundSessionInvalid
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
		return strings.TrimSpace(report.MainUserID) != "" && len(report.BitProfileIDs) > 0 && allNonEmpty(report.BitProfileIDs)
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
