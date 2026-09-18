package service

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/model"
	"github.com/wt-media/wt-media-cloud/internal/shared/id"
)

type (
	ProxyProtocol   = model.ProxyProtocol
	BusinessStatus  = model.BusinessStatus
	ProxySourceType = model.ProxySourceType
	ProxyConfig     = model.ProxyConfig

	CreateProxyInput = dto.CreateProxyInput
	BulkImportRow    = dto.BulkImportRow
	ProxyFilter      = dto.ProxyFilter
)

const (
	ProtocolHTTP      = model.ProtocolHTTP
	ProtocolHTTPS     = model.ProtocolHTTPS
	ProtocolSOCKS5    = model.ProtocolSOCKS5
	BizActive         = model.BizActive
	BizPaused         = model.BizPaused
	BizExpired        = model.BizExpired
	ProxySourceStatic = model.ProxySourceStatic
	ProxySourceAPI    = model.ProxySourceAPI

	DefaultMaxProfiles = 3
)

var (
	ErrInvalidInput  = errors.New("proxy input is invalid")
	ErrNotFound      = errors.New("proxy was not found")
	ErrNotAssignable = errors.New("proxy is not assignable")
)

type Store interface {
	Create(ProxyConfig) error
	FindByID(id string) (ProxyConfig, bool, error)
	List(filter ProxyFilter) ([]ProxyConfig, error)
	Update(ProxyConfig) error
	Delete(id string) error
}

type proxyService struct {
	store Store
	newID func(string) string
	now   func() time.Time
}

func newProxyService(store Store) *proxyService {
	return &proxyService{
		store: store,
		newID: id.NewID,
		now:   time.Now,
	}
}

// BulkParse parses raw proxy lines without persisting.
func (s *proxyService) BulkParse(lines []string) []BulkImportRow {
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
func (s *proxyService) BulkImport(rows []BulkImportRow) ([]ProxyConfig, error) {
	var results []ProxyConfig
	for _, row := range rows {
		if row.Parsed == nil {
			continue
		}
		now := s.now()
		p := ProxyConfig{
			ID:              s.newID("proxy"),
			SourceType:      ProxySourceStatic,
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

func (s *proxyService) List(filter ProxyFilter) ([]ProxyConfig, error) {
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 50
	}
	return s.store.List(filter)
}

// ParseAddress converts one operator-provided proxy address into canonical fields
// without creating or updating a proxy record.
func (s *proxyService) ParseAddress(raw string) (CreateProxyInput, error) {
	parsed, err := parseProxyLine(strings.TrimSpace(raw))
	if err != nil {
		return CreateProxyInput{}, err
	}
	return *parsed, nil
}

func (s *proxyService) Get(id string) (ProxyConfig, error) {
	p, ok, err := s.store.FindByID(id)
	if err != nil {
		return ProxyConfig{}, err
	}
	if !ok {
		return ProxyConfig{}, ErrNotFound
	}
	return p, nil
}

func (s *proxyService) Create(input CreateProxyInput) (ProxyConfig, error) {
	sourceType, extractURL, err := normalizedSource(input.SourceType, input.ExtractURL, "")
	if err != nil {
		return ProxyConfig{}, err
	}
	now := s.now()
	p := ProxyConfig{
		ID:              s.newID("proxy"),
		SourceType:      sourceType,
		ProxyProtocol:   input.ProxyProtocol,
		Host:            input.Host,
		Port:            input.Port,
		Username:        input.Username,
		Password:        input.Password,
		Region:          input.Region,
		Supplier:        input.Supplier,
		ExtractURL:      extractURL,
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
func (s *proxyService) CreateDiscovered(input CreateProxyInput, observedProfileCount int) (ProxyConfig, error) {
	now := s.now()
	p := ProxyConfig{
		ID:              s.newID("proxy"),
		SourceType:      ProxySourceStatic,
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

func (s *proxyService) Update(id string, input CreateProxyInput) (ProxyConfig, error) {
	p, ok, err := s.store.FindByID(id)
	if err != nil {
		return ProxyConfig{}, err
	}
	if !ok {
		return ProxyConfig{}, ErrNotFound
	}
	username := input.Username
	if username == "" {
		username = p.Username
	}
	password := input.Password
	if password == "" {
		password = p.Password
	}
	sourceType, extractURL, err := normalizedSource(input.SourceType, input.ExtractURL, p.ExtractURL)
	if err != nil {
		return ProxyConfig{}, err
	}
	connectionChanged := p.ProxyProtocol != input.ProxyProtocol || p.Host != input.Host || p.Port != input.Port || p.Username != username || p.Password != password
	p.SourceType = sourceType
	p.ProxyProtocol = input.ProxyProtocol
	p.Host = input.Host
	p.Port = input.Port
	p.Username = username
	p.Password = password
	p.Region = input.Region
	p.Supplier = input.Supplier
	p.ExtractURL = extractURL
	if input.ExpiresAt != nil {
		p.ExpiresAt = input.ExpiresAt
	}
	p.Remark = input.Remark
	if connectionChanged {
		p.LastCheckAt = nil
		p.LastCheckResult = ""
		p.ObservedExitIP = ""
	}
	p.UpdatedAt = s.now()
	if err := s.store.Update(p); err != nil {
		return ProxyConfig{}, err
	}
	return p, nil
}

func normalizedSource(source ProxySourceType, extractURL, existingURL string) (ProxySourceType, string, error) {
	if source == "" {
		source = ProxySourceStatic
	}
	if source != ProxySourceStatic && source != ProxySourceAPI {
		return "", "", ErrInvalidInput
	}
	if source == ProxySourceStatic {
		return source, "", nil
	}
	extractURL = strings.TrimSpace(extractURL)
	if extractURL == "" {
		extractURL = existingURL
	}
	if extractURL == "" {
		return "", "", ErrInvalidInput
	}
	return source, extractURL, nil
}

func (s *proxyService) UpdateStatus(id string, status BusinessStatus) (ProxyConfig, error) {
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

func (s *proxyService) Delete(id string) error {
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
func (s *proxyService) TriggerCheck(id string) (ProxyConfig, error) {
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
	addr := proxyDialAddress(host, port)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return fmt.Sprintf("unreachable: %v", err)
	}
	conn.Close()
	return "reachable"
}

func proxyDialAddress(host string, port int) string {
	return net.JoinHostPort(host, strconv.Itoa(port))
}

// CheckQuota verifies a proxy has cross-platform capacity.
func (s *proxyService) CheckQuota(proxyID string, currentAssigned int) (bool, error) {
	proxy, ok, err := s.store.FindByID(proxyID)
	if err != nil {
		return false, err
	}
	if !ok || proxy.BusinessStatus != BizActive {
		return false, nil
	}
	return currentAssigned < normalizedMaxProfileCount(proxy.MaxProfileCount), nil
}

func (s *proxyService) CheckAssignable(proxy ProxyConfig) error {
	if proxy.BusinessStatus != BizActive || proxy.LastCheckResult != "ok" {
		return ErrNotAssignable
	}
	if proxy.ExpiresAt != nil && !proxy.ExpiresAt.After(s.now()) {
		return ErrNotAssignable
	}
	return nil
}

func (s *proxyService) SetMaxProfileCount(proxyID string, maxProfiles int) (ProxyConfig, error) {
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

// NormalizedMaxProfileCount applies the module's default quota at transport boundaries.
func NormalizedMaxProfileCount(value int) int { return normalizedMaxProfileCount(value) }

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
