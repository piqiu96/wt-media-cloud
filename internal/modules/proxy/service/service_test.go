package service

import (
	"errors"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/dto"
	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

func TestProxyDialAddressSupportsIPv6(t *testing.T) {
	if got := proxyDialAddress("2001:db8::1", 1080); got != "[2001:db8::1]:1080" {
		t.Fatalf("proxyDialAddress() = %q", got)
	}
}

func TestParseProxyLineAcceptsBothOperatorFormats(t *testing.T) {
	cases := []struct {
		raw      string
		protocol Protocol
		host     string
		port     int
		user     string
		pass     string
	}{
		{raw: "socks5://alice:secret@10.0.0.1:1080", protocol: ProtocolSOCKS5, host: "10.0.0.1", port: 1080, user: "alice", pass: "secret"},
		{raw: "10.0.0.2:8080:alice:secret", protocol: ProtocolHTTP, host: "10.0.0.2", port: 8080, user: "alice", pass: "secret"},
		{raw: "10.0.0.3:8000", protocol: ProtocolHTTP, host: "10.0.0.3", port: 8000},
	}
	for _, tc := range cases {
		parsed, err := parseProxyLine(tc.raw)
		if err != nil {
			t.Fatalf("parseProxyLine(%q) error = %v", tc.raw, err)
		}
		if parsed.Protocol != tc.protocol || parsed.Host != tc.host || parsed.Port != tc.port ||
			parsed.Username != tc.user || parsed.Password != tc.pass {
			t.Fatalf("parseProxyLine(%q) = %#v", tc.raw, parsed)
		}
	}
	for _, raw := range []string{"", "bad", "ftp://10.0.0.1:21", "10.0.0.1:70000"} {
		if _, err := parseProxyLine(raw); err == nil {
			t.Fatalf("parseProxyLine(%q) = nil error, want invalid", raw)
		}
	}
}

func TestCreateRequiresTeamAndNormalizesTags(t *testing.T) {
	store := newFakeStore()
	svc := newTestProxyService(store)
	if _, err := svc.Create(CreateProxyInput{Name: "a", Host: "10.0.0.1", Port: 8080}, 0); !errors.Is(err, ErrTeamRequired) {
		t.Fatalf("Create without team error = %v", err)
	}
	created, err := svc.Create(CreateProxyInput{
		Name: "  香港-01 ", Host: "10.0.0.1", Port: 8080, Tags: []string{"直播", "直播", " 备用 "},
	}, 7)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.TeamID != 7 || created.Name != "香港-01" || !created.Enabled {
		t.Fatalf("created = %#v", created)
	}
	if len(created.Tags) != 2 || created.Tags[0] != "备用" || created.Tags[1] != "直播" {
		t.Fatalf("tags = %v, want trimmed, de-duplicated, sorted", created.Tags)
	}
	if created.MaxProfiles != DefaultMaxProfiles {
		t.Fatalf("max profile count = %d", created.MaxProfiles)
	}
}

func TestDynamicResourceCanBeRegisteredBeforeItsFirstExtraction(t *testing.T) {
	store := newFakeStore()
	svc := newTestProxyService(store)
	created, err := svc.Create(CreateProxyInput{
		Name: "动态-01", Source: ProxySourceAPI, ExtractURL: "https://supplier.example.com/extract",
	}, 7)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Host != "" || created.Port != 0 || created.LastCheckResult != "" {
		t.Fatalf("created = %#v, want an unchecked resource without an address", created)
	}
	if reason := AssignmentBlocker(created, time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC), DefaultMaxProfiles); reason != "代理尚未检测" {
		t.Fatalf("AssignmentBlocker() = %q, want unchecked", reason)
	}
	if _, err := svc.Create(CreateProxyInput{Name: "静态-无地址"}, 7); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("static proxy without address error = %v", err)
	}
	if _, err := svc.Create(CreateProxyInput{Name: "动态-无提取", Source: ProxySourceAPI, Host: "10.0.0.1", Port: 8080}, 7); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("dynamic proxy without extract URL error = %v", err)
	}
}

func TestOperatorUpdateCannotChangeConnection(t *testing.T) {
	store := newFakeStore()
	store.seed(Proxy{ID: "proxy-1", TeamID: 7, Name: "旧名称", Host: "10.0.0.1", Port: 8080, Protocol: ProtocolHTTP, Enabled: true, MaxProfiles: 3, LastCheckResult: CheckOK})
	svc := newTestProxyService(store)

	updated, err := svc.Update("proxy-1", CreateProxyInput{Name: "新名称", Host: "10.9.9.9", Port: 1, Remark: "备注"}, false)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Name != "新名称" || updated.Remark != "备注" {
		t.Fatalf("updated = %#v", updated)
	}
	if updated.Host != "10.0.0.1" || updated.Port != 8080 {
		t.Fatalf("operator changed connection: %s:%d", updated.Host, updated.Port)
	}
	if updated.LastCheckResult != CheckOK {
		t.Fatalf("operator profile edit cleared the check result: %q", updated.LastCheckResult)
	}
}

func TestGovernedEditKeepsTheQuotaWhenTheFormOmitsIt(t *testing.T) {
	store := newFakeStore()
	store.seed(Proxy{ID: "proxy-1", TeamID: 7, Name: "旧名称", Source: ProxySourceAPI, ExtractURL: "https://supplier.example.com/extract", Host: "10.0.0.1", Port: 8080, Protocol: ProtocolHTTP, Enabled: true, MaxProfiles: 9})
	svc := newTestProxyService(store)

	updated, err := svc.Update("proxy-1", CreateProxyInput{
		Name: "新名称", Source: ProxySourceAPI, Host: "10.0.0.1", Port: 8080, Protocol: ProtocolHTTP,
	}, true)
	if err != nil {
		t.Fatalf("Update() error = %v, want the stored extract URL to satisfy the dynamic source", err)
	}
	if updated.MaxProfiles != 9 {
		t.Fatalf("max profile count = %d, want the stored quota", updated.MaxProfiles)
	}
	if updated.ExtractURL != "https://supplier.example.com/extract" {
		t.Fatalf("extract URL = %q, want the stored one", updated.ExtractURL)
	}
}

func TestGovernedConnectionChangeInvalidatesPreviousCheck(t *testing.T) {
	store := newFakeStore()
	store.seed(Proxy{ID: "proxy-1", TeamID: 7, Name: "旧名称", Host: "10.0.0.1", Port: 8080, Protocol: ProtocolHTTP, Enabled: true, MaxProfiles: 3, LastCheckResult: CheckOK})
	svc := newTestProxyService(store)

	updated, err := svc.Update("proxy-1", CreateProxyInput{Name: "新名称", Host: "10.0.0.2", Port: 8080, Protocol: ProtocolHTTP}, true)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Host != "10.0.0.2" || updated.LastCheckResult != "" || updated.LastCheckAt != nil {
		t.Fatalf("updated = %#v, want new address with previous check cleared", updated)
	}
}

func TestBulkParseKeepsOriginalLineNumbersAndExplainsDuplicates(t *testing.T) {
	rows := BulkParse([]string{
		"10.0.0.1:8080",
		"",
		"10.0.0.1:8080",
		"not-a-proxy",
		"10.0.0.9:1080",
	}, []StoredConnectionKey{{ID: "proxy-existing", Protocol: "http", Host: "10.0.0.9", Port: 1080}})

	if len(rows) != 4 {
		t.Fatalf("rows = %d, want 4 non-empty lines", len(rows))
	}
	if rows[0].Line != 1 || rows[0].Duplicate || rows[0].Error != "" {
		t.Fatalf("row 1 = %#v", rows[0])
	}
	if !rows[1].Duplicate || rows[1].DuplicateReason != "与第 1 行重复" {
		t.Fatalf("row 2 = %#v", rows[1])
	}
	if rows[2].Error == "" {
		t.Fatalf("row 3 = %#v, want invalid address", rows[2])
	}
	if !rows[3].Duplicate || rows[3].DuplicateReason != "已存在代理 #proxy-existing" {
		t.Fatalf("row 4 = %#v", rows[3])
	}
}

func TestImportAccountsForEverySubmittedRow(t *testing.T) {
	store := newFakeStore()
	svc := newTestProxyService(store)
	result, err := svc.Import([]ImportRowInput{
		{Line: 1, Raw: "10.0.0.1:8080", Selected: true},
		{Line: 2, Raw: "10.0.0.2:8080", Selected: false},
		{Line: 3, Raw: "10.0.0.3:8080", Selected: true},
		{Line: 4, Raw: "10.0.0.1:8080", Selected: true},
	}, 7, nil)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if result.Total != 4 || result.Imported != 2 || result.Skipped != 2 || result.Failed != 0 {
		t.Fatalf("result = %#v, want 2 imported / 2 skipped and no failure", result)
	}
	if result.Imported+result.Skipped+result.Failed != result.Total {
		t.Fatalf("counts do not add up: %#v", result)
	}
	if result.Rows[1].Reason != "未选择" {
		t.Fatalf("unselected reason = %q", result.Rows[1].Reason)
	}
	if result.Rows[3].Reason == "" {
		t.Fatalf("duplicate row reason is empty: %#v", result.Rows[3])
	}
	if len(store.created) != 2 || store.created[0].TeamID != 7 {
		t.Fatalf("created = %#v", store.created)
	}
}

func TestAssignmentBlockerReportsTheFirstRealReason(t *testing.T) {
	now := time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
	expired := now.Add(-time.Hour)
	cases := []struct {
		proxy     Proxy
		remaining int
		want      string
	}{
		{proxy: Proxy{Enabled: false, LastCheckResult: CheckOK}, remaining: 1, want: "代理已停用"},
		{proxy: Proxy{Enabled: true, ExpiresAt: &expired, LastCheckResult: CheckOK}, remaining: 1, want: "代理已到期"},
		{proxy: Proxy{Enabled: true}, remaining: 1, want: "代理尚未检测"},
		{proxy: Proxy{Enabled: true, LastCheckResult: CheckAuthFailed}, remaining: 1, want: "代理认证失败"},
		{proxy: Proxy{Enabled: true, LastCheckResult: CheckOK}, remaining: 0, want: "代理窗口配额已满"},
		{proxy: Proxy{Enabled: true, LastCheckResult: CheckOK}, remaining: 1, want: ""},
	}
	for index, tc := range cases {
		if got := AssignmentBlocker(tc.proxy, now, tc.remaining); got != tc.want {
			t.Fatalf("case %d: AssignmentBlocker() = %q, want %q", index, got, tc.want)
		}
	}
	// 检测成功不会让已到期资源复活。
	if !(Proxy{ExpiresAt: &expired}).Expired(now) {
		t.Fatal("Expired() = false for a past date")
	}
}

func TestRecordCheckResultReportsWhenTheResultWasDropped(t *testing.T) {
	store := newFakeStore()
	store.seed(Proxy{ID: "proxy-1", TeamID: 7, Enabled: true, Host: "10.0.0.1", Port: 8080})
	store.checkApplied = false
	svc := newTestProxyService(store)

	_, applied, err := svc.RecordCheckResult("proxy-1", dto.ProxyCheckInput{Protocol: ProtocolHTTP, Host: "10.0.0.1", Port: 8080}, CheckOK, "")
	if err != nil {
		t.Fatalf("RecordCheckResult() error = %v", err)
	}
	if applied {
		t.Fatal("applied = true, want the store's version guard to be reported")
	}
}

func TestWithAssignmentLockTranslatesStoreTimeout(t *testing.T) {
	store := newFakeStore()
	store.lockTimeout = true
	svc := newTestProxyService(store)
	if err := svc.WithAssignmentLock("proxy-1", func() error { return nil }); !errors.Is(err, ErrAssignmentLocked) {
		t.Fatalf("WithAssignmentLock() error = %v", err)
	}
}

// --- 测试替身 ---

type fakeStore struct {
	proxies      map[string]Proxy
	existing     []StoredConnectionKey
	created      []Proxy
	checkApplied bool
	lockTimeout  bool
}

func newFakeStore() *fakeStore {
	return &fakeStore{proxies: make(map[string]Proxy), checkApplied: true}
}

func (f *fakeStore) seed(proxy Proxy) { f.proxies[proxy.ID] = proxy }

func (f *fakeStore) Create(proxy Proxy) error {
	f.proxies[proxy.ID] = proxy
	f.created = append(f.created, proxy)
	return nil
}

func (f *fakeStore) FindByID(id string) (Proxy, bool, error) {
	proxy, ok := f.proxies[id]
	return proxy, ok, nil
}

func (f *fakeStore) List(ProxyFilter) ([]Proxy, error) {
	results := make([]Proxy, 0, len(f.proxies))
	for _, proxy := range f.proxies {
		results = append(results, proxy)
	}
	return results, nil
}

func (f *fakeStore) Update(proxy Proxy) error {
	f.proxies[proxy.ID] = proxy
	return nil
}

func (f *fakeStore) Delete(id string) error {
	delete(f.proxies, id)
	return nil
}

func (f *fakeStore) ListConnectionKeys(sharedidentity.TeamID) ([]StoredConnectionKey, error) {
	return f.existing, nil
}

func (f *fakeStore) SetMaxProfiles(id string, maxProfileCount int, now time.Time) (bool, error) {
	proxy, ok := f.proxies[id]
	if !ok {
		return false, nil
	}
	proxy.MaxProfiles = maxProfileCount
	proxy.UpdatedAt = now
	f.proxies[id] = proxy
	return true, nil
}

func (f *fakeStore) RecordCheckResult(id, protocol, host string, port int, result, message string, checkedAt time.Time) (bool, error) {
	if !f.checkApplied {
		return false, nil
	}
	proxy, ok := f.proxies[id]
	if !ok {
		return false, nil
	}
	proxy.LastCheckAt = &checkedAt
	proxy.LastCheckResult = result
	proxy.UpdatedAt = checkedAt
	f.proxies[id] = proxy
	return true, nil
}

func (f *fakeStore) WithAssignmentLock(string, func() error) error {
	if f.lockTimeout {
		return errStoreLockTimeout
	}
	return nil
}

func newTestProxyService(store Store) *proxyService {
	svc := newProxyService(store)
	svc.now = func() time.Time { return time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC) }
	svc.newID = func(prefix string) string { return prefix + "-generated" }
	return svc
}

func TestAssignmentBlockerForExistingBindingOnlyWaivesQuota(t *testing.T) {
	now := time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
	expired := now.Add(-time.Hour)
	cases := []struct {
		name    string
		proxy   Proxy
		current bool
		want    string
	}{
		{"existing full", Proxy{Enabled: true, LastCheckResult: CheckOK}, true, ""},
		{"new full", Proxy{Enabled: true, LastCheckResult: CheckOK}, false, "代理窗口配额已满"},
		{"existing paused", Proxy{Enabled: false, LastCheckResult: CheckOK}, true, "代理已停用"},
		{"existing expired", Proxy{Enabled: true, ExpiresAt: &expired, LastCheckResult: CheckOK}, true, "代理已到期"},
		{"existing unchecked", Proxy{Enabled: true}, true, "代理尚未检测"},
		{"existing failed", Proxy{Enabled: true, LastCheckResult: CheckAuthFailed}, true, "代理认证失败"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := AssignmentBlockerForProfile(tc.proxy, now, 0, tc.current); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
