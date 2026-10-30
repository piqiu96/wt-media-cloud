package service

import (
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/model"
	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

type (
	Protocol    = model.Protocol
	ProxySource = model.ProxySource
	Proxy       = model.Proxy

	CreateProxyInput                 = dto.CreateProxyInput
	BulkImportRow                    = dto.BulkImportRow
	ProxyFilter                      = dto.ProxyFilter
	ImportRowInput                   = dto.ImportRowInput
	ImportRowResult                  = dto.ImportRowResult
	ImportResult                     = dto.ImportResult
	ProfileProxyOperationInput       = dto.ProfileProxyOperationInput
	ProfileProxyOperationResultInput = dto.ProfileProxyOperationResultInput
	ProfileProxyOperationStart       = dto.ProfileProxyOperationStart
)

const (
	ProtocolHTTP      = model.ProtocolHTTP
	ProtocolHTTPS     = model.ProtocolHTTPS
	ProtocolSOCKS5    = model.ProtocolSOCKS5
	ProxySourceStatic = model.ProxySourceStatic
	ProxySourceAPI    = model.ProxySourceAPI

	CheckOK         = model.CheckOK
	CheckConnFailed = model.CheckConnFailed
	CheckAuthFailed = model.CheckAuthFailed
	CheckTimeout    = model.CheckTimeout
	CheckInvalid    = model.CheckInvalid

	DefaultMaxProfiles = 3
)

var (
	ErrInvalidInput            = errors.New("proxy input is invalid")
	ErrNotFound                = errors.New("proxy was not found")
	ErrNotAssignable           = errors.New("proxy is not assignable")
	ErrTeamRequired            = errors.New("proxy team is required")
	ErrQuotaBelowUsage         = errors.New("proxy quota is below current usage")
	ErrAssignmentLocked        = errors.New("proxy assignment is busy")
	ErrExtractionSourceChanged = errors.New("proxy extraction source changed")
)

// StoredConnectionKey 映射一条已保存的代理连接，让导入预览与提交对
// 「已存在」使用同一判据。
type StoredConnectionKey struct {
	ID       string
	Protocol string
	Host     string
	Port     int
}

type Store interface {
	Create(*Proxy) error
	FindByID(id string) (Proxy, bool, error)
	List(ProxyFilter) ([]Proxy, error)
	Update(Proxy) error
	UpdateExtractedConnection(id, expectedURL string, expectedUpdatedAt time.Time, connection Proxy, now time.Time) (bool, error)
	Delete(id string) error
	ListConnectionKeys(teamID sharedidentity.TeamID) ([]StoredConnectionKey, error)
	SetMaxProfiles(id string, maxProfileCount int, now time.Time) (bool, error)
	RecordCheckResult(id, protocol, host string, port int, username, password, result, message string, checkedAt time.Time) (bool, error)
	WithAssignmentLock(proxyID string, fn func() error) error
}

type proxyService struct {
	store Store
	now   func() time.Time
}

func newProxyService(store Store) *proxyService {
	return &proxyService{
		store: store,
		now:   time.Now,
	}
}

// connectionKey 是团队内查重的规范化标识：协议、主机和端口决定身份，
// 凭据不参与。
func connectionKey(protocol, host string, port int) string {
	return fmt.Sprintf("%s://%s:%d", strings.ToLower(strings.TrimSpace(protocol)), strings.ToLower(strings.TrimSpace(host)), port)
}

func storedKeys(keys []StoredConnectionKey) map[string]string {
	byKey := make(map[string]string, len(keys))
	for _, key := range keys {
		byKey[connectionKey(key.Protocol, key.Host, key.Port)] = key.ID
	}
	return byKey
}

// BulkParse 只解析不落库。行号以调用方提交的非空行为基准从 1 开始，重复项
// 同时对比更早的行和团队已有资源。
func (s *proxyService) BulkParse(lines []string, existingKeys []StoredConnectionKey) []BulkImportRow {
	existing := storedKeys(existingKeys)
	seen := make(map[string]int)
	var results []BulkImportRow
	line := 0
	for _, raw := range lines {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		line++
		row := BulkImportRow{Line: line, Raw: raw}
		parsed, err := parseProxyLine(raw)
		if err != nil {
			row.Error = err.Error()
			results = append(results, row)
			continue
		}
		row.Parsed = parsed
		key := connectionKey(string(parsed.Protocol), parsed.Host, parsed.Port)
		if earlier, duplicate := seen[key]; duplicate {
			row.Duplicate = true
			row.DuplicateReason = fmt.Sprintf("与第 %d 行重复", earlier)
			results = append(results, row)
			continue
		}
		if existingID, duplicate := existing[key]; duplicate {
			row.Duplicate = true
			row.DuplicateReason = fmt.Sprintf("已存在代理 #%s", existingID)
			results = append(results, row)
			continue
		}
		seen[key] = line
		results = append(results, row)
	}
	return results
}

// Import 只创建已勾选、合法且不重复的行。未勾选和非法行计为跳过，保证六类
// 计数之和等于提交总数。
func (s *proxyService) Import(rows []ImportRowInput, teamID sharedidentity.TeamID, existingKeys []StoredConnectionKey) (ImportResult, error) {
	if teamID <= 0 {
		return ImportResult{}, ErrTeamRequired
	}
	existing := storedKeys(existingKeys)
	result := ImportResult{Total: len(rows), Rows: make([]ImportRowResult, 0, len(rows))}
	for _, row := range rows {
		item := ImportRowResult{Line: row.Line, Raw: row.Raw}
		raw := strings.TrimSpace(row.Raw)
		if raw == "" {
			item.Status, item.Reason = dto.ImportStatusSkipped, "空行"
			result.Skipped++
		} else if !row.Selected {
			item.Status, item.Reason = dto.ImportStatusSkipped, "未选择"
			result.Skipped++
		} else if parsed, err := parseProxyLine(raw); err != nil {
			item.Status, item.Reason = dto.ImportStatusSkipped, err.Error()
			result.Skipped++
		} else {
			parsed.Name = strings.TrimSpace(row.Name)
			parsed.Region = strings.TrimSpace(row.Region)
			parsed.Supplier = strings.TrimSpace(row.Supplier)
			parsed.MaxProfiles = row.MaxProfiles
			parsed.Remark = strings.TrimSpace(row.Remark)
			key := connectionKey(string(parsed.Protocol), parsed.Host, parsed.Port)
			if existingID, duplicate := existing[key]; duplicate {
				item.Status, item.Reason = dto.ImportStatusSkipped, fmt.Sprintf("已存在代理 #%s", existingID)
				result.Skipped++
			} else {
				if strings.TrimSpace(parsed.Name) == "" {
					// 导入行只有连接信息；在运营改名之前，地址是唯一可信的默认名称。
					parsed.Name = fmt.Sprintf("%s:%d", parsed.Host, parsed.Port)
				}
				created, createErr := s.create(*parsed, teamID)
				if createErr != nil {
					item.Status, item.Reason = dto.ImportStatusFailed, createErr.Error()
					result.Failed++
				} else {
					item.Status, item.ProxyID = dto.ImportStatusImported, sharedidentity.ProxyID(created.ID)
					existing[key] = created.ID
					result.Imported++
				}
			}
		}
		result.Rows = append(result.Rows, item)
	}
	return result, nil
}

func (s *proxyService) List(filter ProxyFilter) ([]Proxy, error) {
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

func (s *proxyService) Get(id string) (Proxy, error) {
	p, ok, err := s.store.FindByID(id)
	if err != nil {
		return Proxy{}, err
	}
	if !ok {
		return Proxy{}, ErrNotFound
	}
	return p, nil
}

func (s *proxyService) Create(input CreateProxyInput, teamID sharedidentity.TeamID) (Proxy, error) {
	return s.create(input, teamID)
}

func (s *proxyService) create(input CreateProxyInput, teamID sharedidentity.TeamID) (Proxy, error) {
	if teamID <= 0 {
		return Proxy{}, ErrTeamRequired
	}
	normalized, err := normalizeCreateInput(input, true)
	if err != nil {
		return Proxy{}, err
	}
	now := s.now()
	p := Proxy{
		TeamID:      teamID,
		Name:        normalized.Name,
		Source:      normalized.Source,
		Protocol:    normalized.Protocol,
		Host:        normalized.Host,
		Port:        normalized.Port,
		Username:    normalized.Username,
		Password:    normalized.Password,
		Region:      normalized.Region,
		Supplier:    normalized.Supplier,
		Tags:        normalized.Tags,
		ExtractURL:  normalized.ExtractURL,
		ExpiresAt:   normalized.ExpiresAt,
		Enabled:     true,
		MaxProfiles: normalized.MaxProfiles,
		Remark:      normalized.Remark,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.store.Create(&p); err != nil {
		return Proxy{}, err
	}
	return p, nil
}

// CreateDiscovered records a proxy observed in a trusted local Profile scan.
// It is paused until an operator supplements and checks it, so it cannot be
// selected by normal allocation flows merely because it was observed locally.
func (s *proxyService) CreateDiscovered(input CreateProxyInput, teamID sharedidentity.TeamID, observedProfileCount int) (Proxy, error) {
	if teamID <= 0 {
		return Proxy{}, ErrTeamRequired
	}
	now := s.now()
	p := Proxy{
		TeamID:      teamID,
		Name:        fmt.Sprintf("%s:%d", input.Host, input.Port),
		Source:      ProxySourceStatic,
		Protocol:    input.Protocol,
		Host:        input.Host,
		Port:        input.Port,
		Enabled:     false,
		MaxProfiles: max(DefaultMaxProfiles, observedProfileCount),
		Remark:      "本机扫描发现，待补充并检测",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.store.Create(&p); err != nil {
		return Proxy{}, err
	}
	return p, nil
}

// Update 要么应用完整资源配置（治理），要么只改运营维护的字段。团队归属永不
// 迁移：资源属于创建时的团队，已有窗口关系不会跨团队。
func (s *proxyService) Update(id string, input CreateProxyInput, govern bool) (Proxy, error) {
	p, ok, err := s.store.FindByID(id)
	if err != nil {
		return Proxy{}, err
	}
	if !ok {
		return Proxy{}, ErrNotFound
	}
	if !govern {
		p.Name = strings.TrimSpace(input.Name)
		p.Region = strings.TrimSpace(input.Region)
		p.Supplier = strings.TrimSpace(input.Supplier)
		p.Tags = normalizeTags(input.Tags)
		p.Remark = strings.TrimSpace(input.Remark)
		if err := validateName(p.Name); err != nil {
			return Proxy{}, err
		}
		p.UpdatedAt = s.now()
		if err := s.store.Update(p); err != nil {
			return Proxy{}, err
		}
		return p, nil
	}

	requestedMaxProfiles := input.MaxProfiles
	normalized, err := normalizeCreateInput(input, false)
	if err != nil {
		return Proxy{}, err
	}
	if requestedMaxProfiles <= 0 {
		// 编辑资源事实不能顺手重置配额；配额由专门动作修改，并重新检查当前占用。
		normalized.MaxProfiles = p.MaxProfiles
	}
	username := normalized.Username
	if username == "" {
		username = p.Username
	}
	password := normalized.Password
	if password == "" {
		password = p.Password
	}
	if input.ReplaceCredentials {
		username = normalized.Username
		password = normalized.Password
	}
	extractURL := normalized.ExtractURL
	if extractURL == "" {
		extractURL = p.ExtractURL
	}
	connectionChanged := p.Protocol != normalized.Protocol || p.Host != normalized.Host ||
		p.Port != normalized.Port || p.Username != username || p.Password != password
	p.Name = normalized.Name
	p.Source = normalized.Source
	p.Protocol = normalized.Protocol
	p.Host = normalized.Host
	p.Port = normalized.Port
	p.Username = username
	p.Password = password
	p.Region = normalized.Region
	p.Supplier = normalized.Supplier
	p.Tags = normalized.Tags
	p.ExtractURL = extractURL
	if normalized.ExpiresAt != nil {
		p.ExpiresAt = normalized.ExpiresAt
	}
	p.Remark = normalized.Remark
	p.MaxProfiles = normalized.MaxProfiles
	if govern && normalized.Enabled != nil {
		p.Enabled = *normalized.Enabled
	}
	if connectionChanged {
		// 旧结果属于上一个连接版本；保留它会让新地址凭旧的成功记录变成可分配。
		p.LastCheckAt = nil
		p.LastCheckResult = ""
		p.LastCheckMessage = ""
	}
	p.UpdatedAt = s.now()
	if err := s.store.Update(p); err != nil {
		return Proxy{}, err
	}
	return p, nil
}

// RefreshExtractedConnection 只写本次提取的连接字段；名称、配额、启停和备注
// 可能在本机请求期间被其他人编辑，不能用读取时的旧值覆盖。
func (s *proxyService) RefreshExtractedConnection(id, expectedURL string, expectedUpdatedAt time.Time, input CreateProxyInput) (Proxy, error) {
	if strings.TrimSpace(expectedURL) == "" || expectedUpdatedAt.IsZero() || strings.TrimSpace(input.Host) == "" || input.Port < 1 || input.Port > 65535 {
		return Proxy{}, ErrInvalidInput
	}
	switch input.Protocol {
	case ProtocolHTTP, ProtocolHTTPS, ProtocolSOCKS5:
	default:
		return Proxy{}, ErrInvalidInput
	}
	connection := Proxy{
		Protocol: input.Protocol,
		Host:     strings.TrimSpace(input.Host),
		Port:     input.Port,
		Username: strings.TrimSpace(input.Username),
		Password: input.Password,
	}
	applied, err := s.store.UpdateExtractedConnection(id, expectedURL, expectedUpdatedAt, connection, s.now())
	if err != nil {
		return Proxy{}, err
	}
	if !applied {
		return Proxy{}, ErrExtractionSourceChanged
	}
	return s.Get(id)
}

// normalizeCreateInput 校验创建与更新共用的写入结构。creating 保留仅创建时
// 的要求（动态来源必须给出提取 URL）；更新可以沿用已保存的 URL 而不回传。
func normalizeCreateInput(input CreateProxyInput, creating bool) (CreateProxyInput, error) {
	normalized := input
	normalized.Name = strings.TrimSpace(input.Name)
	normalized.Host = strings.TrimSpace(input.Host)
	normalized.Region = strings.TrimSpace(input.Region)
	normalized.Supplier = strings.TrimSpace(input.Supplier)
	normalized.Remark = strings.TrimSpace(input.Remark)
	normalized.Username = strings.TrimSpace(input.Username)
	normalized.Tags = normalizeTags(input.Tags)
	if input.MaxProfiles <= 0 {
		normalized.MaxProfiles = DefaultMaxProfiles
	}
	if err := validateName(normalized.Name); err != nil {
		return CreateProxyInput{}, err
	}
	switch normalized.Protocol {
	case ProtocolHTTP, ProtocolHTTPS, ProtocolSOCKS5:
	case "":
		normalized.Protocol = ProtocolHTTP
	default:
		return CreateProxyInput{}, fmt.Errorf("%w: 不支持的代理协议", ErrInvalidInput)
	}
	if normalized.Source == "" {
		normalized.Source = ProxySourceStatic
	}
	if normalized.Source != ProxySourceStatic && normalized.Source != ProxySourceAPI {
		return CreateProxyInput{}, fmt.Errorf("%w: 不支持的来源类型", ErrInvalidInput)
	}
	normalized.ExtractURL = strings.TrimSpace(input.ExtractURL)
	if creating && normalized.Source == ProxySourceAPI && normalized.ExtractURL == "" {
		return CreateProxyInput{}, fmt.Errorf("%w: 动态代理需要提取地址", ErrInvalidInput)
	}
	// 动态资源可以先登记、后首次提取。提取到地址之前保持未检测，也就不可分配。
	if !(normalized.Source == ProxySourceAPI && normalized.Host == "" && normalized.Port == 0) {
		if normalized.Host == "" {
			return CreateProxyInput{}, fmt.Errorf("%w: 地址不能为空", ErrInvalidInput)
		}
		if normalized.Port < 1 || normalized.Port > 65535 {
			return CreateProxyInput{}, fmt.Errorf("%w: 端口必须是 1-65535", ErrInvalidInput)
		}
	}
	if normalized.MaxProfiles < 1 {
		return CreateProxyInput{}, fmt.Errorf("%w: 最大窗口数必须是正整数", ErrInvalidInput)
	}
	return normalized, nil
}

func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: 名称不能为空", ErrInvalidInput)
	}
	if len([]rune(name)) > 64 {
		return fmt.Errorf("%w: 名称最多 64 个字符", ErrInvalidInput)
	}
	return nil
}

func normalizeTags(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	normalized := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, duplicate := seen[tag]; duplicate {
			continue
		}
		seen[tag] = struct{}{}
		normalized = append(normalized, tag)
	}
	sort.Strings(normalized)
	return normalized
}

func (s *proxyService) UpdateEnabled(id string, enabled bool) (Proxy, error) {
	p, ok, err := s.store.FindByID(id)
	if err != nil {
		return Proxy{}, err
	}
	if !ok {
		return Proxy{}, ErrNotFound
	}
	p.Enabled = enabled
	p.UpdatedAt = s.now()
	if err := s.store.Update(p); err != nil {
		return Proxy{}, err
	}
	return p, nil
}

func (s *proxyService) UpdateGovernance(id string, enabled *bool, maxProfiles *int) (Proxy, error) {
	p, ok, err := s.store.FindByID(id)
	if err != nil {
		return Proxy{}, err
	}
	if !ok {
		return Proxy{}, ErrNotFound
	}
	if enabled != nil {
		p.Enabled = *enabled
	}
	if maxProfiles != nil {
		if *maxProfiles < 1 {
			return Proxy{}, ErrInvalidInput
		}
		p.MaxProfiles = *maxProfiles
	}
	p.UpdatedAt = s.now()
	if err := s.store.Update(p); err != nil {
		return Proxy{}, err
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

// SetMaxProfiles 只校验数值本身。占用数由调用方在分配锁内检查，
// 因为占用关系统计在另一个模块。
func (s *proxyService) SetMaxProfiles(proxyID string, maxProfiles int) (Proxy, error) {
	if maxProfiles < 1 {
		return Proxy{}, fmt.Errorf("%w: 最大窗口数必须是正整数", ErrInvalidInput)
	}
	ok, err := s.store.SetMaxProfiles(proxyID, maxProfiles, s.now())
	if err != nil {
		return Proxy{}, err
	}
	if !ok {
		return Proxy{}, ErrNotFound
	}
	return s.Get(proxyID)
}

func (s *proxyService) WithAssignmentLock(proxyID string, fn func() error) error {
	err := s.store.WithAssignmentLock(proxyID, fn)
	if errors.Is(err, errStoreLockTimeout) {
		return ErrAssignmentLocked
	}
	return err
}

// errStoreLockTimeout 由存储适配器转出，服务层因此不依赖 repository 包。
var errStoreLockTimeout = errors.New("proxy assignment lock timed out")

// AssignmentBlocker 说明资源为何不可分配；空串表示全部前置条件成立。
// remaining 是剩余可用名额。
func AssignmentBlocker(p Proxy, now time.Time, remaining int) string {
	if !p.Enabled {
		return "代理已停用"
	}
	if p.Expired(now) {
		return "代理已到期"
	}
	switch p.LastCheckResult {
	case CheckOK:
	case CheckAuthFailed:
		return "代理认证失败"
	case CheckTimeout:
		return "代理检测超时"
	case CheckConnFailed:
		return "代理连接失败"
	case "":
		return "代理尚未检测"
	default:
		return "代理检测结果不可用"
	}
	if remaining <= 0 {
		return "代理窗口配额已满"
	}
	return ""
}

// An existing binding consumes no additional slot, but must still pass every
// other availability check before its configuration can be written again.
func AssignmentBlockerForProfile(p Proxy, now time.Time, remaining int, alreadyAssigned bool) string {
	if alreadyAssigned && remaining < 1 {
		remaining = 1
	}
	return AssignmentBlocker(p, now, remaining)
}

func (s *proxyService) CheckAssignable(p Proxy) error {
	if AssignmentBlocker(p, s.now(), 1) != "" {
		return ErrNotAssignable
	}
	return nil
}

func (s *proxyService) CheckQuota(proxyID string, currentAssigned int) (bool, error) {
	proxy, ok, err := s.store.FindByID(proxyID)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	if AssignmentBlocker(proxy, s.now(), normalizedMaxProfiles(proxy.MaxProfiles)-currentAssigned) != "" {
		return false, nil
	}
	return true, nil
}

func normalizedMaxProfiles(value int) int {
	if value <= 0 {
		return DefaultMaxProfiles
	}
	return value
}

// NormalizedMaxProfiles 是对外展示用的同一规则。
func NormalizedMaxProfiles(value int) int {
	return normalizedMaxProfiles(value)
}

// RecordCheckResult 仅在探测时的连接版本仍是当前版本时保存结果。针对已改连接
// 的迟到结果会被丢弃，不覆盖新版本状态。
func (s *proxyService) RecordCheckResult(id string, checked dto.ProxyCheckInput, result, message string) (Proxy, bool, error) {
	applied, err := s.store.RecordCheckResult(id, string(checked.Protocol), checked.Host, checked.Port, checked.Username, checked.Password, result, message, s.now())
	if err != nil {
		return Proxy{}, false, err
	}
	p, ok, err := s.store.FindByID(id)
	if err != nil {
		return Proxy{}, false, err
	}
	if !ok {
		return Proxy{}, false, ErrNotFound
	}
	return p, applied, nil
}

// Probe 在服务端执行连通性检测，不需要本机 Agent、浏览器窗口或比特连接。
func (s *proxyService) Probe(p Proxy, opts ProbeOptions) dto.ProxyCheckResult {
	return Probe(p.Protocol, p.Host, p.Port, p.Username, p.Password, opts)
}

func proxyDialAddress(host string, port int) string {
	return net.JoinHostPort(host, strconv.Itoa(port))
}
