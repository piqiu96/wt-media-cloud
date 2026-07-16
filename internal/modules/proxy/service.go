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
	ProtocolHTTP  ProxyProtocol = "http"
	ProtocolHTTPS ProxyProtocol = "https"
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
	ID              string         `json:"id"`
	ProxyProtocol   ProxyProtocol  `json:"proxy_protocol"`
	Host            string         `json:"host"`
	Port            int            `json:"port"`
	Username        string         `json:"username,omitempty"`
	Password        string         `json:"password,omitempty"`
	Region          string         `json:"region,omitempty"`
	Supplier        string         `json:"supplier,omitempty"`
	ExpiresAt       *time.Time     `json:"expires_at,omitempty"`
	BusinessStatus  BusinessStatus `json:"business_status"`
	LastCheckAt     *time.Time     `json:"last_check_at,omitempty"`
	LastCheckResult string         `json:"last_check_result,omitempty"`
	ObservedExitIP  string         `json:"observed_exit_ip,omitempty"`
	Remark          string         `json:"remark,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	PlatformQuotas  []PlatformQuota `json:"platform_quotas,omitempty"`
}

type PlatformQuota struct {
	ProxyID     string `json:"proxy_id"`
	Platform    string `json:"platform"`
	MaxProfiles int    `json:"max_profiles"`
}

type CreateProxyInput struct {
	ProxyProtocol ProxyProtocol `json:"proxy_protocol"`
	Host          string        `json:"host"`
	Port          int           `json:"port"`
	Username      string        `json:"username"`
	Password      string        `json:"password"`
	Region        string        `json:"region"`
	Supplier      string        `json:"supplier"`
	ExpiresAt     *time.Time    `json:"expires_at"`
	Remark        string        `json:"remark"`
}

type BulkImportRow struct {
	Raw     string `json:"raw"`
	Parsed  *CreateProxyInput `json:"parsed,omitempty"`
	Error   string `json:"error,omitempty"`
}

type Store interface {
	Create(ProxyConfig) error
	FindByID(id string) (ProxyConfig, bool, error)
	List(filter ProxyFilter) ([]ProxyConfig, error)
	Update(ProxyConfig) error
	Delete(id string) error
	UpsertQuota(PlatformQuota) error
	ListQuotas(proxyID string) ([]PlatformQuota, error)
	DeleteQuota(proxyID, platform string) error
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
			ID:             s.newID("proxy"),
			ProxyProtocol:  row.Parsed.ProxyProtocol,
			Host:           row.Parsed.Host,
			Port:           row.Parsed.Port,
			Username:       row.Parsed.Username,
			Password:       row.Parsed.Password,
			Region:         row.Parsed.Region,
			Supplier:       row.Parsed.Supplier,
			ExpiresAt:      row.Parsed.ExpiresAt,
			BusinessStatus: BizActive,
			Remark:         row.Parsed.Remark,
			CreatedAt:      now,
			UpdatedAt:      now,
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
	quotas, _ := s.store.ListQuotas(id)
	p.PlatformQuotas = quotas
	return p, nil
}

func (s *Service) Create(input CreateProxyInput) (ProxyConfig, error) {
	now := s.now()
	p := ProxyConfig{
		ID:             s.newID("proxy"),
		ProxyProtocol:  input.ProxyProtocol,
		Host:           input.Host,
		Port:           input.Port,
		Username:       input.Username,
		Password:       input.Password,
		Region:         input.Region,
		Supplier:       input.Supplier,
		ExpiresAt:      input.ExpiresAt,
		BusinessStatus: BizActive,
		Remark:         input.Remark,
		CreatedAt:      now,
		UpdatedAt:      now,
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
func (s *Service) TriggerCheck(id string) error {
	proxy, ok, err := s.store.FindByID(id)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	now := s.now()
	result := checkTCPConnect(proxy.Host, proxy.Port)
	proxy.LastCheckAt = &now
	proxy.LastCheckResult = result
	if err := s.store.Update(proxy); err != nil {
		return err
	}
	return nil
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

// SetQuota creates or updates a per-platform quota for a proxy.
// DefaultMaxProfiles is the default limit when no per-proxy quota is configured.
const DefaultMaxProfiles = 3

// CheckQuota verifies a proxy has capacity for the given platform.
// currentAssigned is the number of profiles already using this proxy for the platform.
func (s *Service) CheckQuota(proxyID, platform string, currentAssigned int) (bool, error) {
	proxy, ok, err := s.store.FindByID(proxyID)
	if err != nil {
		return false, err
	}
	if !ok || proxy.BusinessStatus != BizActive {
		return false, nil
	}
	quotas, err := s.store.ListQuotas(proxyID)
	if err != nil {
		return false, err
	}
	maxProfiles := DefaultMaxProfiles
	for _, q := range quotas {
		if q.Platform == platform {
			maxProfiles = q.MaxProfiles
			break
		}
	}
	return currentAssigned < maxProfiles, nil
}

func (s *Service) SetQuota(proxyID, platform string, maxProfiles int) (PlatformQuota, error) {
	_, ok, err := s.store.FindByID(proxyID)
	if err != nil {
		return PlatformQuota{}, err
	}
	if !ok {
		return PlatformQuota{}, ErrNotFound
	}
	q := PlatformQuota{
		ProxyID:     proxyID,
		Platform:    platform,
		MaxProfiles: maxProfiles,
	}
	if err := s.store.UpsertQuota(q); err != nil {
		return PlatformQuota{}, err
	}
	return q, nil
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
