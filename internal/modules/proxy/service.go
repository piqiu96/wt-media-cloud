package proxy

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/common"
)

type ProxyProtocol string

const (
	ProtocolHTTP   ProxyProtocol = "http"
	ProtocolHTTPS  ProxyProtocol = "https"
	ProtocolSOCKS5 ProxyProtocol = "socks5"
)

type BusinessStatus string

const (
	BizActive  BusinessStatus = "active"
	BizPaused  BusinessStatus = "paused"
	BizExpired BusinessStatus = "expired"
)

var (
	ErrInvalidInput = errors.New("proxy input is invalid")
	ErrNotFound     = errors.New("proxy was not found")
)

type ProxyConfig struct {
	ID                    string         `json:"id"`
	ProxyProtocol         ProxyProtocol  `json:"proxy_protocol"`
	Host                  string         `json:"host"`
	Port                  int            `json:"port"`
	Username              string         `json:"username,omitempty"`
	Password              string         `json:"password,omitempty"`
	Region                string         `json:"region,omitempty"`
	Supplier              string         `json:"supplier,omitempty"`
	ExpiresAt             *time.Time     `json:"expires_at,omitempty"`
	BusinessStatus        BusinessStatus `json:"business_status"`
	MaxProfileCount       int            `json:"max_profile_count"`
	LastCheckAt           *time.Time     `json:"last_check_at,omitempty"`
	LastCheckResult       string         `json:"last_check_result,omitempty"`
	ObservedExitIP        string         `json:"observed_exit_ip,omitempty"`
	AssignedProfileCount  int            `json:"assigned_profile_count,omitempty"`
	RemainingProfileCount int            `json:"remaining_profile_count,omitempty"`
	Remark                string         `json:"remark,omitempty"`
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
}

type CreateProxyInput struct {
	ProxyProtocol   ProxyProtocol `json:"proxy_protocol"`
	Host            string        `json:"host"`
	Port            int           `json:"port"`
	Username        string        `json:"username"`
	Password        string        `json:"password"`
	Region          string        `json:"region"`
	Supplier        string        `json:"supplier"`
	ExpiresAt       *time.Time    `json:"expires_at"`
	Remark          string        `json:"remark"`
	MaxProfileCount int           `json:"max_profile_count"`
}

type BulkImportRow struct {
	Raw    string            `json:"raw"`
	Parsed *CreateProxyInput `json:"parsed,omitempty"`
	Error  string            `json:"error,omitempty"`
}

type Store interface {
	Create(ProxyConfig) error
	FindByID(id string) (ProxyConfig, bool, error)
	List(filter ProxyFilter) ([]ProxyConfig, error)
	Update(ProxyConfig) error
	Delete(id string) error
}

type Service struct {
	store Store
	newID func(string) string
	now   func() time.Time
}

func NewService(store Store) *Service {
	return &Service{
		store: store,
		newID: common.NewID,
		now:   time.Now,
	}
}

// BulkParse parses raw proxy lines without persisting.
func (s *Service) BulkParse(lines []string) []BulkImportRow {
	var results []BulkImportRow
	for _, raw := range lines {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		row := BulkImportRow{Raw: raw}
		parsed, err := parseProxyLine(raw)
		if err != nil {
			row.Error = err.Error()
		} else {
			row.Parsed = parsed
		}
		results = append(results, row)
	}
	return results
}

// BulkImport validates and persists parsed proxy lines.
func (s *Service) BulkImport(rows []BulkImportRow) ([]ProxyConfig, error) {
	var results []ProxyConfig
	for _, row := range rows {
		if row.Parsed == nil {
			continue
		}
		now := s.now()
		p := ProxyConfig{
			ID:              s.newID("proxy"),
			ProxyProtocol:   row.Parsed.ProxyProtocol,
			Host:            row.Parsed.Host,
			Port:            row.Parsed.Port,
			Username:        row.Parsed.Username,
			Password:        row.Parsed.Password,
			Region:          row.Parsed.Region,
			Supplier:        row.Parsed.Supplier,
			ExpiresAt:       row.Parsed.ExpiresAt,
			BusinessStatus:  BizActive,
			MaxProfileCount: normalizedMaxProfileCount(row.Parsed.MaxProfileCount),
			Remark:          row.Parsed.Remark,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := s.store.Create(p); err != nil {
			return results, err
		}
		results = append(results, p)
	}
	return results, nil
}

func (s *Service) List(filter ProxyFilter) ([]ProxyConfig, error) {
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 50
	}
	return s.store.List(filter)
}

func (s *Service) Get(id string) (ProxyConfig, error) {
	p, ok, err := s.store.FindByID(id)
	if err != nil {
		return ProxyConfig{}, err
	}
	if !ok {
		return ProxyConfig{}, ErrNotFound
	}
	return p, nil
}

func (s *Service) Create(input CreateProxyInput) (ProxyConfig, error) {
	now := s.now()
	p := ProxyConfig{
		ID:              s.newID("proxy"),
		ProxyProtocol:   input.ProxyProtocol,
		Host:            input.Host,
		Port:            input.Port,
		Username:        input.Username,
		Password:        input.Password,
		Region:          input.Region,
		Supplier:        input.Supplier,
		ExpiresAt:       input.ExpiresAt,
		BusinessStatus:  BizActive,
		MaxProfileCount: normalizedMaxProfileCount(input.MaxProfileCount),
		Remark:          input.Remark,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.store.Create(p); err != nil {
		return ProxyConfig{}, err
	}
	return p, nil
}

// CreateDiscovered records a proxy observed in a trusted local Profile scan.
// It is paused until an operator supplements and checks it, so it cannot be
// selected by normal allocation flows merely because it was observed locally.
func (s *Service) CreateDiscovered(input CreateProxyInput, observedProfileCount int) (ProxyConfig, error) {
	now := s.now()
	p := ProxyConfig{
		ID:              s.newID("proxy"),
		ProxyProtocol:   input.ProxyProtocol,
		Host:            input.Host,
		Port:            input.Port,
		BusinessStatus:  BizPaused,
		MaxProfileCount: max(DefaultMaxProfiles, observedProfileCount),
		Remark:          "本机扫描发现，待补充并检测",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.store.Create(p); err != nil {
		return ProxyConfig{}, err
	}
	return p, nil
}

func (s *Service) Update(id string, input CreateProxyInput) (ProxyConfig, error) {
	p, ok, err := s.store.FindByID(id)
	if err != nil {
		return ProxyConfig{}, err
	}
	if !ok {
		return ProxyConfig{}, ErrNotFound
	}
	p.ProxyProtocol = input.ProxyProtocol
	p.Host = input.Host
	p.Port = input.Port
	p.Username = input.Username
	p.Password = input.Password
	p.Region = input.Region
	p.Supplier = input.Supplier
	p.ExpiresAt = input.ExpiresAt
	p.Remark = input.Remark
	p.UpdatedAt = s.now()
	if err := s.store.Update(p); err != nil {
		return ProxyConfig{}, err
	}
	return p, nil
}

func (s *Service) UpdateStatus(id string, status BusinessStatus) (ProxyConfig, error) {
	p, ok, err := s.store.FindByID(id)
	if err != nil {
		return ProxyConfig{}, err
	}
	if !ok {
		return ProxyConfig{}, ErrNotFound
	}
	p.BusinessStatus = status
	p.UpdatedAt = s.now()
	if err := s.store.Update(p); err != nil {
		return ProxyConfig{}, err
	}
	return p, nil
}

func (s *Service) Delete(id string) error {
	_, ok, err := s.store.FindByID(id)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return s.store.Delete(id)
}

// TriggerCheck performs a basic TCP connectivity check on the proxy.
// Full protocol-level check requires Agent-side execution.
func (s *Service) TriggerCheck(id string) (ProxyConfig, error) {
	p, ok, err := s.store.FindByID(id)
	if err != nil {
		return ProxyConfig{}, err
	}
	if !ok {
		return ProxyConfig{}, ErrNotFound
	}
	now := s.now()
	result := checkTCPConnect(p.Host, p.Port)
	p.LastCheckAt = &now
	p.LastCheckResult = result
	if err := s.store.Update(p); err != nil {
		return ProxyConfig{}, err
	}
	return p, nil
}

// checkTCPConnect attempts a basic TCP dial to validate host:port reachability.
func checkTCPConnect(host string, port int) string {
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return fmt.Sprintf("unreachable: %v", err)
	}
	conn.Close()
	return "reachable"
}

const DefaultMaxProfiles = 3

// CheckQuota verifies a proxy has cross-platform capacity.
func (s *Service) CheckQuota(proxyID string, currentAssigned int) (bool, error) {
	proxy, ok, err := s.store.FindByID(proxyID)
	if err != nil {
		return false, err
	}
	if !ok || proxy.BusinessStatus != BizActive {
		return false, nil
	}
	return currentAssigned < normalizedMaxProfileCount(proxy.MaxProfileCount), nil
}

func (s *Service) SetMaxProfileCount(proxyID string, maxProfiles int) (ProxyConfig, error) {
	proxy, ok, err := s.store.FindByID(proxyID)
	if err != nil {
		return ProxyConfig{}, err
	}
	if !ok {
		return ProxyConfig{}, ErrNotFound
	}
	if maxProfiles <= 0 || maxProfiles > 1000 {
		return ProxyConfig{}, ErrInvalidInput
	}
	proxy.MaxProfileCount = maxProfiles
	proxy.UpdatedAt = s.now()
	if err := s.store.Update(proxy); err != nil {
		return ProxyConfig{}, err
	}
	return proxy, nil
}

func normalizedMaxProfileCount(value int) int {
	if value <= 0 {
		return DefaultMaxProfiles
	}
	return value
}

// --- Helpers ---

type ProxyFilter struct {
	Platform       string `json:"platform"`
	Supplier       string `json:"supplier"`
	BusinessStatus string `json:"business_status"`
	Region         string `json:"region"`
	Search         string `json:"search"`
	Limit          int    `json:"limit"`
	Offset         int    `json:"offset"`
}

func parseProxyLine(raw string) (*CreateProxyInput, error) {
	// Format 1: protocol://user:pass@host:port
	if strings.Contains(raw, "://") {
		return parseURLProxy(raw)
	}
	// Format 2: host:port:user:pass
	parts := strings.Split(raw, ":")
	if len(parts) >= 2 {
		return parseColonProxy(parts)
	}
	return nil, errors.New("unrecognized proxy format")
}

func parseURLProxy(raw string) (*CreateProxyInput, error) {
	// Simple parse: extract protocol, user:pass, host:port
	rest := raw
	proto := ProtocolHTTP
	if idx := strings.Index(rest, "://"); idx > 0 {
		proto = ProxyProtocol(rest[:idx])
		rest = rest[idx+3:]
	}

	userInfo, hostPort, ok := strings.Cut(rest, "@")
	if !ok {
		userInfo = ""
		hostPort = rest
	}

	host, portStr, _ := strings.Cut(hostPort, ":")
	port := 0
	if portStr != "" {
		port = parseInt(portStr)
	}

	username, password, _ := strings.Cut(userInfo, ":")

	if host == "" || port == 0 {
		return nil, errors.New("missing host or port")
	}

	return &CreateProxyInput{
		ProxyProtocol: proto,
		Host:          host,
		Port:          port,
		Username:      username,
		Password:      password,
	}, nil
}

func parseColonProxy(parts []string) (*CreateProxyInput, error) {
	if len(parts) < 2 {
		return nil, errors.New("need at least host:port")
	}
	host := parts[0]
	port := parseInt(parts[1])
	if host == "" || port == 0 {
		return nil, errors.New("invalid host or port")
	}
	proxy := &CreateProxyInput{
		ProxyProtocol: ProtocolHTTP,
		Host:          host,
		Port:          port,
	}
	if len(parts) >= 4 {
		proxy.Username = parts[2]
		proxy.Password = parts[3]
	}
	return proxy, nil
}

func parseInt(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			return 0
		}
	}
	return n
}
