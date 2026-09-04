package proxy

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
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
func (s *routeMemoryStore) List(ProxyFilter) ([]ProxyConfig, error) { return nil, nil }
func (s *routeMemoryStore) Update(proxy ProxyConfig) error          { s.items[proxy.ID] = proxy; return nil }
func (s *routeMemoryStore) Delete(id string) error                  { delete(s.items, id); return nil }
