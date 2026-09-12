package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

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

func TestParseProxyAddressReturnsCanonicalFieldsWithoutPersisting(t *testing.T) {
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

	response := proxyJSON(engine, "POST", "/api/v1/proxies/parse", `{"proxy_address":"socks5://operator:secret@203.0.113.9:1080"}`, login.Token)
	if response.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("parse status=%d body=%s", response.Result().StatusCode(), response.Result().Body())
	}
	var envelope struct {
		Data CreateProxyInput `json:"data"`
	}
	if err := json.Unmarshal(response.Result().Body(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.ProxyProtocol != ProtocolSOCKS5 || envelope.Data.Host != "203.0.113.9" || envelope.Data.Port != 1080 || envelope.Data.Username != "operator" || envelope.Data.Password != "secret" {
		t.Fatalf("parsed data=%#v", envelope.Data)
	}
	if len(store.items) != 0 {
		t.Fatalf("parse persisted proxy data: %#v", store.items)
	}
}

func TestProxyListMasksDynamicSourceAndCredentials(t *testing.T) {
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

	created := proxyJSON(engine, "POST", "/api/v1/proxies", `{"source_type":"api","extract_url":"https://provider.example/extract?token=source-secret","proxy_protocol":"socks5","host":"203.0.113.9","port":1080,"username":"proxy-user","password":"proxy-secret"}`, login.Token)
	if created.Result().StatusCode() != consts.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.Result().StatusCode(), created.Result().Body())
	}
	listed := proxyJSON(engine, "GET", "/api/v1/proxies", "", login.Token)
	if listed.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("list status=%d body=%s", listed.Result().StatusCode(), listed.Result().Body())
	}
	body := string(listed.Result().Body())
	for _, secret := range []string{"source-secret", "proxy-secret", "proxy-user"} {
		if bytes.Contains(listed.Result().Body(), []byte(secret)) {
			t.Fatalf("proxy list leaked secret %q: %s", secret, body)
		}
	}
	if !bytes.Contains(listed.Result().Body(), []byte(`"source_type":"api"`)) || !bytes.Contains(listed.Result().Body(), []byte(`"extract_url_configured":true`)) {
		t.Fatalf("dynamic source summary missing: %s", body)
	}
}

func TestDynamicProxyPreviewRequiresLocalExtractor(t *testing.T) {
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

	response := proxyJSON(engine, "POST", "/api/v1/proxies/extract-preview", `{"extract_url":"https://provider.example/extract"}`, login.Token)
	if response.Result().StatusCode() != consts.StatusServiceUnavailable {
		t.Fatalf("preview status=%d body=%s", response.Result().StatusCode(), response.Result().Body())
	}
}

func TestProxyBindingDetailRequiresProfileVisibilityService(t *testing.T) {
	store := newRouteMemoryStore()
	store.items["proxy-1"] = ProxyConfig{ID: "proxy-1", SourceType: ProxySourceStatic}
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

	response := proxyJSON(engine, "GET", "/api/v1/proxies/proxy-1/bindings", "", login.Token)
	if response.Result().StatusCode() != consts.StatusServiceUnavailable {
		t.Fatalf("binding detail status=%d body=%s", response.Result().StatusCode(), response.Result().Body())
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

func TestAgentReachableCheckResultMakesProxyAssignable(t *testing.T) {
	store := newRouteMemoryStore()
	store.items["proxy-1"] = ProxyConfig{ID: "proxy-1", ProxyProtocol: ProtocolSOCKS5, Host: "127.0.0.1", Port: 1080, BusinessStatus: BizActive, MaxProfileCount: 2}
	service := NewService(store)

	updated, err := service.RecordCheckResult("proxy-1", "reachable")
	if err != nil {
		t.Fatal(err)
	}
	if updated.LastCheckResult != "ok" {
		t.Fatalf("agent reachable result stored as %q, want normalized ok", updated.LastCheckResult)
	}
	if err := service.CheckAssignable(updated); err != nil {
		t.Fatalf("reachable proxy rejected after normalization: %v", err)
	}
}

func TestUpdateConnectionClearsPreviousCheckResult(t *testing.T) {
	store := newRouteMemoryStore()
	checkedAt := time.Now().UTC()
	expiresAt := checkedAt.Add(24 * time.Hour)
	store.items["proxy-1"] = ProxyConfig{
		ID: "proxy-1", ProxyProtocol: ProtocolHTTP, Host: "10.0.0.1", Port: 8080, Password: "existing-secret",
		BusinessStatus: BizActive, ExpiresAt: &expiresAt, LastCheckAt: &checkedAt, LastCheckResult: "ok", ObservedExitIP: "10.0.0.1",
	}
	service := NewService(store)

	updated, err := service.Update("proxy-1", CreateProxyInput{ProxyProtocol: ProtocolHTTP, Host: "10.0.0.2", Port: 8080})
	if err != nil {
		t.Fatal(err)
	}
	if updated.LastCheckAt != nil || updated.LastCheckResult != "" || updated.ObservedExitIP != "" {
		t.Fatalf("connection update retained stale check facts: %#v", updated)
	}
	if updated.Password != "existing-secret" {
		t.Fatalf("empty edit password cleared the stored secret: %#v", updated)
	}
	if updated.ExpiresAt == nil || !updated.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("edit without expiry cleared the stored expiry: %#v", updated)
	}
}

func TestDeleteRefusesProxyWithBoundProfiles(t *testing.T) {
	store := newRouteMemoryStore()
	store.items["proxy-1"] = ProxyConfig{ID: "proxy-1", BusinessStatus: BizActive}
	identityService := identity.NewService(identity.NewMemoryStore())
	if _, err := identityService.BootstrapAdmin("admin", "a-long-initial-password"); err != nil {
		t.Fatal(err)
	}
	login, err := identityService.Login("admin", "a-long-initial-password")
	if err != nil {
		t.Fatal(err)
	}
	profiles := &routeProfileStore{profileCount: 1}
	engine := server.New()
	identity.RegisterRoutes(engine, identityService, identity.RouteConfig{CookieSecure: false})
	RegisterRoutes(engine, NewService(store), identityService, profiles)

	response := proxyJSON(engine, "DELETE", "/api/v1/proxies/proxy-1", "", login.Token)
	if response.Result().StatusCode() != consts.StatusConflict {
		t.Fatalf("delete status=%d body=%s", response.Result().StatusCode(), response.Result().Body())
	}
	if _, found := store.items["proxy-1"]; !found {
		t.Fatal("bound proxy was deleted")
	}
}

func TestAssignWritesThroughAgentThenBindsReadbackProfile(t *testing.T) {
	store := newRouteMemoryStore()
	store.items["proxy-1"] = ProxyConfig{ID: "proxy-1", ProxyProtocol: ProtocolSOCKS5, Host: "127.0.0.1", Port: 1080, BusinessStatus: BizActive, LastCheckResult: "ok", MaxProfileCount: 2}
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

func TestAssignRejectsProxyWithoutSuccessfulCheck(t *testing.T) {
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
	if response.Result().StatusCode() != consts.StatusConflict {
		t.Fatalf("assign status=%d body=%s", response.Result().StatusCode(), response.Result().Body())
	}
}

func TestAssignReplacesExistingProxyOnlyAfterNewReadback(t *testing.T) {
	store := newRouteMemoryStore()
	store.items["proxy-new"] = ProxyConfig{ID: "proxy-new", ProxyProtocol: ProtocolHTTP, Host: "10.0.0.2", Port: 8080, BusinessStatus: BizActive, LastCheckResult: "ok", MaxProfileCount: 2}
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

func TestBatchAssignMutatesEachAuthorizedProfileAndReturnsReadback(t *testing.T) {
	store := newRouteMemoryStore()
	store.items["proxy-1"] = ProxyConfig{ID: "proxy-1", ProxyProtocol: ProtocolSOCKS5, Host: "127.0.0.1", Port: 1080, BusinessStatus: BizActive, LastCheckResult: "ok", MaxProfileCount: 2}
	identityService := identity.NewService(identity.NewMemoryStore())
	if _, err := identityService.BootstrapAdmin("admin", "a-long-initial-password"); err != nil {
		t.Fatal(err)
	}
	login, err := identityService.Login("admin", "a-long-initial-password")
	if err != nil {
		t.Fatal(err)
	}
	profiles := &routeProfileStore{profiles: map[string]profilebinding.BrowserProfile{
		"profile-1": {ID: "profile-1", UserID: 1, BitProfileID: "bit-profile-1", LocalStatus: profilebinding.ProfileActive},
		"profile-2": {ID: "profile-2", UserID: 1, BitProfileID: "bit-profile-2", LocalStatus: profilebinding.ProfileActive},
	}}
	mutator := &routeMutator{results: []ProxyMutationResult{
		{ProfileID: "bit-profile-1", ProxyProtocol: ProtocolSOCKS5, Host: "127.0.0.1", Port: 1080, Readback: true},
		{ProfileID: "bit-profile-2", ProxyProtocol: ProtocolSOCKS5, Host: "127.0.0.1", Port: 1080, Readback: true},
	}}
	engine := server.New()
	identity.RegisterRoutes(engine, identityService, identity.RouteConfig{CookieSecure: false})
	RegisterRoutes(engine, NewService(store), identityService, profiles, mutator)

	response := proxyJSON(engine, "POST", "/api/v1/proxies/proxy-1/assign-batch", `{"profile_ids":["profile-1","profile-2"]}`, login.Token)
	if response.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("batch assign status=%d body=%s", response.Result().StatusCode(), response.Result().Body())
	}
	if len(mutator.inputs) != 2 || profiles.profiles["profile-1"].ProxyID != "proxy-1" || profiles.profiles["profile-2"].ProxyID != "proxy-1" {
		t.Fatalf("inputs=%#v profiles=%#v", mutator.inputs, profiles.profiles)
	}
	if !bytes.Contains(response.Result().Body(), []byte(`"succeeded"`)) || bytes.Contains(response.Result().Body(), []byte(`"failed":[{`)) {
		t.Fatalf("batch response=%s", response.Result().Body())
	}
}

func TestBatchAssignKeepsFailedProfileUnbound(t *testing.T) {
	store := newRouteMemoryStore()
	store.items["proxy-1"] = ProxyConfig{ID: "proxy-1", ProxyProtocol: ProtocolSOCKS5, Host: "127.0.0.1", Port: 1080, BusinessStatus: BizActive, LastCheckResult: "ok", MaxProfileCount: 2}
	identityService := identity.NewService(identity.NewMemoryStore())
	if _, err := identityService.BootstrapAdmin("admin", "a-long-initial-password"); err != nil {
		t.Fatal(err)
	}
	login, err := identityService.Login("admin", "a-long-initial-password")
	if err != nil {
		t.Fatal(err)
	}
	profiles := &routeProfileStore{profiles: map[string]profilebinding.BrowserProfile{
		"profile-1": {ID: "profile-1", UserID: 1, BitProfileID: "bit-profile-1", LocalStatus: profilebinding.ProfileActive},
		"profile-2": {ID: "profile-2", UserID: 1, BitProfileID: "bit-profile-2", LocalStatus: profilebinding.ProfileActive},
	}}
	mutator := &routeMutator{results: []ProxyMutationResult{
		{ProfileID: "bit-profile-1", ProxyProtocol: ProtocolSOCKS5, Host: "127.0.0.1", Port: 1080, Readback: true},
		{ProfileID: "bit-profile-2", ProxyProtocol: ProtocolSOCKS5, Host: "127.0.0.1", Port: 1080, Readback: false},
	}}
	engine := server.New()
	identity.RegisterRoutes(engine, identityService, identity.RouteConfig{CookieSecure: false})
	RegisterRoutes(engine, NewService(store), identityService, profiles, mutator)

	response := proxyJSON(engine, "POST", "/api/v1/proxies/proxy-1/assign-batch", `{"profile_ids":["profile-1","profile-2"]}`, login.Token)
	if response.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("batch assign status=%d body=%s", response.Result().StatusCode(), response.Result().Body())
	}
	if profiles.profiles["profile-1"].ProxyID != "proxy-1" || profiles.profiles["profile-2"].ProxyID != "" {
		t.Fatalf("failed batch item changed formal proxy relation: %#v", profiles.profiles)
	}
	if !bytes.Contains(response.Result().Body(), []byte(`"profile_id":"profile-2"`)) {
		t.Fatalf("batch response omitted failed profile=%s", response.Result().Body())
	}
}

func TestProxyRecommendationsRequireCheckedActiveCapacity(t *testing.T) {
	store := newRouteMemoryStore()
	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Hour)
	store.items = map[string]ProxyConfig{
		"recommended": {ID: "recommended", ProxyProtocol: ProtocolSOCKS5, Host: "127.0.0.1", Port: 1080, BusinessStatus: BizActive, LastCheckResult: "ok", ExpiresAt: &future, MaxProfileCount: 2},
		"unchecked":   {ID: "unchecked", ProxyProtocol: ProtocolSOCKS5, Host: "127.0.0.2", Port: 1080, BusinessStatus: BizActive, MaxProfileCount: 2},
		"paused":      {ID: "paused", ProxyProtocol: ProtocolSOCKS5, Host: "127.0.0.3", Port: 1080, BusinessStatus: BizPaused, LastCheckResult: "ok", MaxProfileCount: 2},
		"expired":     {ID: "expired", ProxyProtocol: ProtocolSOCKS5, Host: "127.0.0.4", Port: 1080, BusinessStatus: BizActive, LastCheckResult: "ok", ExpiresAt: &past, MaxProfileCount: 2},
		"full":        {ID: "full", ProxyProtocol: ProtocolSOCKS5, Host: "127.0.0.5", Port: 1080, BusinessStatus: BizActive, LastCheckResult: "ok", ExpiresAt: &future, MaxProfileCount: 1},
	}
	identityService := identity.NewService(identity.NewMemoryStore())
	if _, err := identityService.BootstrapAdmin("admin", "a-long-initial-password"); err != nil {
		t.Fatal(err)
	}
	login, err := identityService.Login("admin", "a-long-initial-password")
	if err != nil {
		t.Fatal(err)
	}
	profiles := &routeProfileStore{profiles: map[string]profilebinding.BrowserProfile{
		"profile-1": {ID: "profile-1", UserID: 1, BitProfileID: "bit-profile-1", LocalStatus: profilebinding.ProfileActive},
		"profile-2": {ID: "profile-2", UserID: 1, BitProfileID: "bit-profile-2", LocalStatus: profilebinding.ProfileActive},
	}}
	engine := server.New()
	identity.RegisterRoutes(engine, identityService, identity.RouteConfig{CookieSecure: false})
	RegisterRoutes(engine, NewService(store), identityService, profiles)

	response := proxyJSON(engine, "GET", "/api/v1/proxies/recommendations?profile_ids=profile-1,profile-2", "", login.Token)
	if response.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("recommendations status=%d body=%s", response.Result().StatusCode(), response.Result().Body())
	}
	body := string(response.Result().Body())
	if !strings.Contains(body, `"id":"recommended"`) {
		t.Fatalf("missing eligible recommendation: %s", body)
	}
	for _, id := range []string{"unchecked", "paused", "expired", "full"} {
		if strings.Contains(body, `"id":"`+id+`"`) {
			t.Fatalf("ineligible proxy %q recommended: %s", id, body)
		}
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
	profiles     map[string]profilebinding.BrowserProfile
	boundProxyID string
	unbound      bool
	profileCount int
}

func (s *routeProfileStore) GetProfile(id string) (profilebinding.BrowserProfile, bool, error) {
	if s.profiles != nil {
		profile, found := s.profiles[id]
		return profile, found, nil
	}
	return s.profile, s.profile.ID == id, nil
}
func (s *routeProfileStore) CountProfilesByProxyID(proxyID string) (int, error) {
	if s.profiles != nil {
		count := 0
		for _, profile := range s.profiles {
			if profile.ProxyID == proxyID {
				count++
			}
		}
		return count, nil
	}
	return s.profileCount, nil
}
func (s *routeProfileStore) BindProxy(profileID, proxyID, proxyType, proxyHost string, proxyPort int) (profilebinding.BrowserProfile, error) {
	if s.profiles != nil {
		profile, found := s.profiles[profileID]
		if !found {
			return profilebinding.BrowserProfile{}, profilebinding.ErrProfileNotFound
		}
		profile.ProxyID, profile.ProxyType, profile.ProxyHost, profile.ProxyPort = proxyID, proxyType, proxyHost, proxyPort
		s.profiles[profileID] = profile
		return profile, nil
	}
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
	input   ProxyMutationInput
	inputs  []ProxyMutationInput
	result  ProxyMutationResult
	results []ProxyMutationResult
}

func (m *routeMutator) Mutate(_ context.Context, input ProxyMutationInput) (ProxyMutationResult, error) {
	m.input = input
	m.inputs = append(m.inputs, input)
	if len(m.results) > 0 {
		result := m.results[0]
		m.results = m.results[1:]
		return result, nil
	}
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
