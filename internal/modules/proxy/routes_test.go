package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
	"github.com/wt-media/wt-media-cloud/internal/modules/profilebinding"
)

func TestImportPreviewDoesNotPersistUntilConfirmed(t *testing.T) {
	store := newRouteMemoryStore()
	identityService := identity.NewService(identity.NewMemoryStore())
	if _, err := identityService.BootstrapAdmin("admin", "a-long-initial-password"); err != nil {
		t.Fatal(err)
	}
	login, err := identityService.Login("admin", "a-long-initial-password")
	if err != nil {
		t.Fatal(err)
	}
	engine := server.New()
	identity.RegisterRoutes(engine, identityService, identity.RouteConfig{CookieSecure: false})
	RegisterRoutes(engine, NewService(store), identityService)

	preview := proxyJSON(engine, "POST", "/api/v1/proxies/import/preview", `{"lines":["http://127.0.0.1:1080"]}`, login.Token)
	if preview.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("preview status=%d body=%s", preview.Result().StatusCode(), preview.Result().Body())
	}
	if len(store.items) != 0 {
		t.Fatalf("preview persisted proxies: %#v", store.items)
	}
	var previewEnvelope struct {
		Data struct {
			Parsed []BulkImportRow `json:"parsed"`
		} `json:"data"`
	}
	if err := json.Unmarshal(preview.Result().Body(), &previewEnvelope); err != nil || len(previewEnvelope.Data.Parsed) != 1 || previewEnvelope.Data.Parsed[0].Parsed == nil {
		t.Fatalf("preview body=%s err=%v", preview.Result().Body(), err)
	}

	confirmed := proxyJSON(engine, "POST", "/api/v1/proxies/import", `{"lines":["http://127.0.0.1:1080"]}`, login.Token)
	if confirmed.Result().StatusCode() != consts.StatusCreated {
		t.Fatalf("confirm status=%d body=%s", confirmed.Result().StatusCode(), confirmed.Result().Body())
	}
	if len(store.items) != 1 {
		t.Fatalf("confirm persisted %d proxies, want 1", len(store.items))
	}
	for _, proxy := range store.items {
		if proxy.MaxProfileCount != DefaultMaxProfiles {
			t.Fatalf("import max_profile_count=%d, want default %d", proxy.MaxProfileCount, DefaultMaxProfiles)
		}
	}
}

func TestUnifiedQuotaAppliesAcrossPlatforms(t *testing.T) {
	store := newRouteMemoryStore()
	store.items["proxy-1"] = ProxyConfig{ID: "proxy-1", BusinessStatus: BizActive, MaxProfileCount: 2}
	service := NewService(store)

	available, err := service.CheckQuota("proxy-1", 1)
	if err != nil || !available {
		t.Fatalf("CheckQuota(1) = %v, %v; want true, nil", available, err)
	}
	available, err = service.CheckQuota("proxy-1", 2)
	if err != nil || available {
		t.Fatalf("CheckQuota(2) = %v, %v; want false, nil", available, err)
	}
	updated, err := service.SetMaxProfileCount("proxy-1", 4)
	if err != nil || updated.MaxProfileCount != 4 {
		t.Fatalf("SetMaxProfileCount() = %#v, %v", updated, err)
	}
}

func TestAssignWritesThroughAgentThenBindsReadbackProfile(t *testing.T) {
	store := newRouteMemoryStore()
	store.items["proxy-1"] = ProxyConfig{ID: "proxy-1", ProxyProtocol: ProtocolSOCKS5, Host: "127.0.0.1", Port: 1080, BusinessStatus: BizActive, MaxProfileCount: 2}
	identityService := identity.NewService(identity.NewMemoryStore())
	if _, err := identityService.BootstrapAdmin("admin", "a-long-initial-password"); err != nil {
		t.Fatal(err)
	}
	login, err := identityService.Login("admin", "a-long-initial-password")
	if err != nil {
		t.Fatal(err)
	}
	profiles := &routeProfileStore{profile: profilebinding.BrowserProfile{ID: "profile-1", UserID: 1, BitProfileID: "bit-profile-1", LocalStatus: profilebinding.ProfileActive}}
	mutator := &routeMutator{result: ProxyMutationResult{ProfileID: "bit-profile-1", ProxyProtocol: ProtocolSOCKS5, Host: "127.0.0.1", Port: 1080, Readback: true}}
	engine := server.New()
	identity.RegisterRoutes(engine, identityService, identity.RouteConfig{CookieSecure: false})
	RegisterRoutes(engine, NewService(store), identityService, profiles, mutator)

	response := proxyJSON(engine, "POST", "/api/v1/proxies/proxy-1/assign", `{"profile_id":"profile-1"}`, login.Token)
	if response.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("assign status=%d body=%s", response.Result().StatusCode(), response.Result().Body())
	}
	if mutator.input.ProfileID != "bit-profile-1" || profiles.boundProxyID != "proxy-1" {
		t.Fatalf("mutation=%#v bound_proxy=%q", mutator.input, profiles.boundProxyID)
	}
	if !bytes.Contains(response.Result().Body(), []byte(`"proxy_id":"proxy-1"`)) {
		t.Fatalf("assign response=%s", response.Result().Body())
	}
}

func TestAssignReplacesExistingProxyOnlyAfterNewReadback(t *testing.T) {
	store := newRouteMemoryStore()
	store.items["proxy-new"] = ProxyConfig{ID: "proxy-new", ProxyProtocol: ProtocolHTTP, Host: "10.0.0.2", Port: 8080, BusinessStatus: BizActive, MaxProfileCount: 2}
	identityService := identity.NewService(identity.NewMemoryStore())
	if _, err := identityService.BootstrapAdmin("admin", "a-long-initial-password"); err != nil {
		t.Fatal(err)
	}
	login, err := identityService.Login("admin", "a-long-initial-password")
	if err != nil {
		t.Fatal(err)
	}
	profiles := &routeProfileStore{profile: profilebinding.BrowserProfile{ID: "profile-1", UserID: 1, BitProfileID: "bit-profile-1", ProxyID: "proxy-old", LocalStatus: profilebinding.ProfileActive}}
	mutator := &routeMutator{result: ProxyMutationResult{ProfileID: "bit-profile-1", ProxyProtocol: ProtocolHTTP, Host: "10.0.0.2", Port: 8080, Readback: true}}
	engine := server.New()
	identity.RegisterRoutes(engine, identityService, identity.RouteConfig{CookieSecure: false})
	RegisterRoutes(engine, NewService(store), identityService, profiles, mutator)

	response := proxyJSON(engine, "POST", "/api/v1/proxies/proxy-new/assign", `{"profile_id":"profile-1"}`, login.Token)
	if response.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("replace status=%d body=%s", response.Result().StatusCode(), response.Result().Body())
	}
	if profiles.boundProxyID != "proxy-new" || mutator.input.ProfileID != "bit-profile-1" {
		t.Fatalf("bound=%q mutation=%#v", profiles.boundProxyID, mutator.input)
	}
}

func TestUnbindClearsFormalProxyOnlyAfterNoProxyReadback(t *testing.T) {
	store := newRouteMemoryStore()
	store.items["proxy-1"] = ProxyConfig{ID: "proxy-1", ProxyProtocol: ProtocolHTTP, Host: "10.0.0.1", Port: 8080, BusinessStatus: BizActive, MaxProfileCount: 2}
	identityService := identity.NewService(identity.NewMemoryStore())
	if _, err := identityService.BootstrapAdmin("admin", "a-long-initial-password"); err != nil {
		t.Fatal(err)
	}
	login, err := identityService.Login("admin", "a-long-initial-password")
	if err != nil {
		t.Fatal(err)
	}
	profiles := &routeProfileStore{profile: profilebinding.BrowserProfile{ID: "profile-1", UserID: 1, BitProfileID: "bit-profile-1", ProxyID: "proxy-1", ProxyType: "http", ProxyHost: "10.0.0.1", ProxyPort: 8080, LocalStatus: profilebinding.ProfileActive}}
	mutator := &routeMutator{result: ProxyMutationResult{ProfileID: "bit-profile-1", Operation: "unbind", Readback: true}}
	engine := server.New()
	identity.RegisterRoutes(engine, identityService, identity.RouteConfig{CookieSecure: false})
	RegisterRoutes(engine, NewService(store), identityService, profiles, mutator)

	response := proxyJSON(engine, "POST", "/api/v1/proxies/proxy-1/unbind", `{"profile_id":"profile-1"}`, login.Token)
	if response.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("unbind status=%d body=%s", response.Result().StatusCode(), response.Result().Body())
	}
	if !profiles.unbound || profiles.profile.ProxyID != "" {
		t.Fatalf("profile was not formally unbound: %#v", profiles.profile)
	}
	if mutator.input.Operation != "unbind" || mutator.input.ProfileID != "bit-profile-1" {
		t.Fatalf("mutation=%#v", mutator.input)
	}
}

func TestLocalProxyScanPreviewShowsKnownChangeWithoutWritingFormalRelation(t *testing.T) {
	store := newRouteMemoryStore()
	store.items["proxy-old"] = ProxyConfig{ID: "proxy-old", ProxyProtocol: ProtocolHTTP, Host: "10.0.0.1", Port: 8080, BusinessStatus: BizActive, MaxProfileCount: 3}
	store.items["proxy-new"] = ProxyConfig{ID: "proxy-new", ProxyProtocol: ProtocolSOCKS5, Host: "10.0.0.2", Port: 1080, BusinessStatus: BizActive, MaxProfileCount: 3}
	identityService := identity.NewService(identity.NewMemoryStore())
	if _, err := identityService.BootstrapAdmin("admin", "a-long-initial-password"); err != nil {
		t.Fatal(err)
	}
	login, err := identityService.Login("admin", "a-long-initial-password")
	if err != nil {
		t.Fatal(err)
	}
	profiles := &routeProfileStore{profile: profilebinding.BrowserProfile{ID: "profile-1", UserID: 1, BitProfileID: "bit-profile-1", ProxyID: "proxy-old", ProxyType: "http", ProxyHost: "10.0.0.1", ProxyPort: 8080, LocalStatus: profilebinding.ProfileActive}}
	scans := &routeScanController{scan: profilebinding.ProfileScan{ID: "scan-1", UserID: 1, Status: profilebinding.ScanReady, Profiles: []profilebinding.BrowserProfile{{ID: "profile-1", UserID: 1, BitProfileID: "bit-profile-1", ProxyType: "socks5", ProxyHost: "10.0.0.2", ProxyPort: 1080}}}}
	engine := server.New()
	identity.RegisterRoutes(engine, identityService, identity.RouteConfig{CookieSecure: false})
	RegisterRoutes(engine, NewService(store), identityService, profiles, scans, routeTrustChecker{})

	response := proxyJSON(engine, "POST", "/api/v1/proxies/local-scan/preview", `{"scan_id":"scan-1"}`, login.Token)
	if response.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("preview status=%d body=%s", response.Result().StatusCode(), response.Result().Body())
	}
	var envelope struct {
		Data struct {
			Changes []struct {
				Kind           string `json:"kind"`
				ProfileID      string `json:"profile_id"`
				CurrentProxyID string `json:"current_proxy_id"`
				TargetProxyID  string `json:"target_proxy_id"`
			} `json:"changes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Result().Body(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data.Changes) != 1 || envelope.Data.Changes[0].Kind != "changed" || envelope.Data.Changes[0].ProfileID != "profile-1" || envelope.Data.Changes[0].CurrentProxyID != "proxy-old" || envelope.Data.Changes[0].TargetProxyID != "proxy-new" {
		t.Fatalf("unexpected preview: %s", response.Result().Body())
	}
	if profiles.profile.ProxyID != "proxy-old" || profiles.boundProxyID != "" || profiles.unbound {
		t.Fatalf("preview changed formal relation: %#v", profiles.profile)
	}
}

func TestLocalProxyScanConfirmAppliesKnownReadbackAsFormalChange(t *testing.T) {
	store := newRouteMemoryStore()
	store.items["proxy-old"] = ProxyConfig{ID: "proxy-old", ProxyProtocol: ProtocolHTTP, Host: "10.0.0.1", Port: 8080, BusinessStatus: BizActive, MaxProfileCount: 3}
	store.items["proxy-new"] = ProxyConfig{ID: "proxy-new", ProxyProtocol: ProtocolSOCKS5, Host: "10.0.0.2", Port: 1080, BusinessStatus: BizActive, MaxProfileCount: 3}
	identityService := identity.NewService(identity.NewMemoryStore())
	if _, err := identityService.BootstrapAdmin("admin", "a-long-initial-password"); err != nil {
		t.Fatal(err)
	}
	login, err := identityService.Login("admin", "a-long-initial-password")
	if err != nil {
		t.Fatal(err)
	}
	profiles := &routeProfileStore{profile: profilebinding.BrowserProfile{ID: "profile-1", UserID: 1, BitProfileID: "bit-profile-1", ProxyID: "proxy-old", ProxyType: "http", ProxyHost: "10.0.0.1", ProxyPort: 8080, LocalStatus: profilebinding.ProfileActive}}
	scans := &routeScanController{scan: profilebinding.ProfileScan{ID: "scan-1", UserID: 1, Status: profilebinding.ScanReady, Profiles: []profilebinding.BrowserProfile{{ID: "profile-1", UserID: 1, BitProfileID: "bit-profile-1", ProxyType: "socks5", ProxyHost: "10.0.0.2", ProxyPort: 1080}}}}
	engine := server.New()
	identity.RegisterRoutes(engine, identityService, identity.RouteConfig{CookieSecure: false})
	RegisterRoutes(engine, NewService(store), identityService, profiles, scans, routeTrustChecker{})

	response := proxyJSON(engine, "POST", "/api/v1/proxies/local-scan/confirm", `{"scan_id":"scan-1","node_id":"node-1"}`, login.Token)
	if response.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("confirm status=%d body=%s", response.Result().StatusCode(), response.Result().Body())
	}
	if profiles.boundProxyID != "proxy-new" || profiles.profile.ProxyID != "proxy-new" || scans.confirmed != 1 {
		t.Fatalf("formal relation not reconciled: profile=%#v confirms=%d", profiles.profile, scans.confirmed)
	}
}

func TestLocalProxyScanConfirmCreatesPausedRecordForUnknownProxy(t *testing.T) {
	store := newRouteMemoryStore()
	identityService := identity.NewService(identity.NewMemoryStore())
	if _, err := identityService.BootstrapAdmin("admin", "a-long-initial-password"); err != nil {
		t.Fatal(err)
	}
	login, err := identityService.Login("admin", "a-long-initial-password")
	if err != nil {
		t.Fatal(err)
	}
	profiles := &routeProfileStore{profile: profilebinding.BrowserProfile{ID: "profile-1", UserID: 1, BitProfileID: "bit-profile-1", LocalStatus: profilebinding.ProfileActive}}
	scans := &routeScanController{scan: profilebinding.ProfileScan{ID: "scan-1", UserID: 1, Status: profilebinding.ScanReady, Profiles: []profilebinding.BrowserProfile{{ID: "profile-1", UserID: 1, BitProfileID: "bit-profile-1", ProxyType: "http", ProxyHost: "10.0.0.8", ProxyPort: 8888}}}}
	engine := server.New()
	identity.RegisterRoutes(engine, identityService, identity.RouteConfig{CookieSecure: false})
	RegisterRoutes(engine, NewService(store), identityService, profiles, scans, routeTrustChecker{})

	response := proxyJSON(engine, "POST", "/api/v1/proxies/local-scan/confirm", `{"scan_id":"scan-1","node_id":"node-1"}`, login.Token)
	if response.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("confirm status=%d body=%s", response.Result().StatusCode(), response.Result().Body())
	}
	if len(store.items) != 1 || profiles.profile.ProxyID == "" || scans.confirmed != 1 {
		t.Fatalf("unknown proxy was not recorded and bound: proxies=%#v profile=%#v confirms=%d", store.items, profiles.profile, scans.confirmed)
	}
	for _, created := range store.items {
		if created.BusinessStatus != BizPaused || created.LastCheckResult != "" || created.Host != "10.0.0.8" || created.Port != 8888 {
			t.Fatalf("unexpected discovered proxy: %#v", created)
		}
	}
}

func TestLocalProxyScanConfirmRequiresCurrentTrustedNode(t *testing.T) {
	store := newRouteMemoryStore()
	identityService := identity.NewService(identity.NewMemoryStore())
	if _, err := identityService.BootstrapAdmin("admin", "a-long-initial-password"); err != nil {
		t.Fatal(err)
	}
	login, err := identityService.Login("admin", "a-long-initial-password")
	if err != nil {
		t.Fatal(err)
	}
	profiles := &routeProfileStore{profile: profilebinding.BrowserProfile{ID: "profile-1", UserID: 1, BitProfileID: "bit-profile-1", LocalStatus: profilebinding.ProfileActive}}
	scans := &routeScanController{scan: profilebinding.ProfileScan{ID: "scan-1", UserID: 1, Status: profilebinding.ScanReady, Profiles: []profilebinding.BrowserProfile{{ID: "profile-1", UserID: 1, BitProfileID: "bit-profile-1", ProxyType: "http", ProxyHost: "10.0.0.8", ProxyPort: 8888}}}}
	engine := server.New()
	identity.RegisterRoutes(engine, identityService, identity.RouteConfig{CookieSecure: false})
	RegisterRoutes(engine, NewService(store), identityService, profiles, scans)

	response := proxyJSON(engine, "POST", "/api/v1/proxies/local-scan/confirm", `{"scan_id":"scan-1"}`, login.Token)
	if response.Result().StatusCode() != consts.StatusBadRequest {
		t.Fatalf("confirm without node status=%d body=%s", response.Result().StatusCode(), response.Result().Body())
	}
}

func TestLocalProxyScanConfirmRetriesFormalReconciliationFromConfirmedScan(t *testing.T) {
	store := newRouteMemoryStore()
	store.items["proxy-new"] = ProxyConfig{ID: "proxy-new", ProxyProtocol: ProtocolHTTP, Host: "10.0.0.2", Port: 8080, BusinessStatus: BizActive, MaxProfileCount: 3}
	identityService := identity.NewService(identity.NewMemoryStore())
	if _, err := identityService.BootstrapAdmin("admin", "a-long-initial-password"); err != nil {
		t.Fatal(err)
	}
	login, err := identityService.Login("admin", "a-long-initial-password")
	if err != nil {
		t.Fatal(err)
	}
	profiles := &routeProfileStore{profile: profilebinding.BrowserProfile{ID: "profile-1", UserID: 1, BitProfileID: "bit-profile-1", LocalStatus: profilebinding.ProfileActive}}
	scans := &routeScanController{scan: profilebinding.ProfileScan{ID: "scan-1", UserID: 1, Status: profilebinding.ScanConfirmed, Profiles: []profilebinding.BrowserProfile{{ID: "profile-1", UserID: 1, BitProfileID: "bit-profile-1", ProxyType: "http", ProxyHost: "10.0.0.2", ProxyPort: 8080}}}, confirmErr: errors.New("already confirmed")}
	engine := server.New()
	identity.RegisterRoutes(engine, identityService, identity.RouteConfig{CookieSecure: false})
	RegisterRoutes(engine, NewService(store), identityService, profiles, scans, routeTrustChecker{})

	response := proxyJSON(engine, "POST", "/api/v1/proxies/local-scan/confirm", `{"scan_id":"scan-1","node_id":"node-1"}`, login.Token)
	if response.Result().StatusCode() != consts.StatusOK || profiles.profile.ProxyID != "proxy-new" {
		t.Fatalf("retry status=%d body=%s profile=%#v", response.Result().StatusCode(), response.Result().Body(), profiles.profile)
	}
}

func proxyJSON(engine *server.Hertz, method, path, body, token string) *ut.ResponseRecorder {
	return ut.PerformRequest(engine.Engine, method, path,
		&ut.Body{Body: bytes.NewBufferString(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
		ut.Header{Key: "X-Session-Token", Value: token},
	)
}

type routeMemoryStore struct{ items map[string]ProxyConfig }

func newRouteMemoryStore() *routeMemoryStore {
	return &routeMemoryStore{items: map[string]ProxyConfig{}}
}

func (s *routeMemoryStore) Create(proxy ProxyConfig) error { s.items[proxy.ID] = proxy; return nil }
func (s *routeMemoryStore) FindByID(id string) (ProxyConfig, bool, error) {
	proxy, ok := s.items[id]
	return proxy, ok, nil
}
func (s *routeMemoryStore) List(ProxyFilter) ([]ProxyConfig, error) {
	items := make([]ProxyConfig, 0, len(s.items))
	for _, item := range s.items {
		items = append(items, item)
	}
	return items, nil
}
func (s *routeMemoryStore) Update(proxy ProxyConfig) error { s.items[proxy.ID] = proxy; return nil }
func (s *routeMemoryStore) Delete(id string) error         { delete(s.items, id); return nil }

type routeProfileStore struct {
	profile      profilebinding.BrowserProfile
	boundProxyID string
	unbound      bool
}

func (s *routeProfileStore) GetProfile(id string) (profilebinding.BrowserProfile, bool, error) {
	return s.profile, s.profile.ID == id, nil
}
func (s *routeProfileStore) CountProfilesByProxyID(proxyID string) (int, error) { return 0, nil }
func (s *routeProfileStore) BindProxy(profileID, proxyID, proxyType, proxyHost string, proxyPort int) (profilebinding.BrowserProfile, error) {
	s.boundProxyID = proxyID
	s.profile.ProxyID, s.profile.ProxyType, s.profile.ProxyHost, s.profile.ProxyPort = proxyID, proxyType, proxyHost, proxyPort
	return s.profile, nil
}
func (s *routeProfileStore) UnbindProxy(profileID, expectedProxyID string) (profilebinding.BrowserProfile, error) {
	if s.profile.ID != profileID || s.profile.ProxyID != expectedProxyID {
		return profilebinding.BrowserProfile{}, profilebinding.ErrProfileNotFound
	}
	s.unbound = true
	s.profile.ProxyID, s.profile.ProxyType, s.profile.ProxyHost, s.profile.ProxyPort = "", "", "", 0
	return s.profile, nil
}

type routeMutator struct {
	input  ProxyMutationInput
	result ProxyMutationResult
}

func (m *routeMutator) Mutate(_ context.Context, input ProxyMutationInput) (ProxyMutationResult, error) {
	m.input = input
	return m.result, nil
}

type routeScanController struct {
	scan       profilebinding.ProfileScan
	confirmed  int
	confirmErr error
}

type routeTrustChecker struct{}

func (routeTrustChecker) CheckLocalTrust(identity.UserID, string) error { return nil }

func (s *routeScanController) GetScan(actor identity.PublicUser, scanID string) (profilebinding.ProfileScan, error) {
	if scanID != s.scan.ID || actor.ID != s.scan.UserID {
		return profilebinding.ProfileScan{}, profilebinding.ErrScanNotFound
	}
	return s.scan, nil
}

func (s *routeScanController) ConfirmScan(actor identity.PublicUser, scanID string) (profilebinding.ProfileScan, error) {
	scan, err := s.GetScan(actor, scanID)
	if err == nil {
		s.confirmed++
	}
	if err == nil && s.confirmErr != nil {
		return profilebinding.ProfileScan{}, s.confirmErr
	}
	return scan, err
}
