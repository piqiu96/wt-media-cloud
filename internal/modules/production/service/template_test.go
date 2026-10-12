package service

import (
	"errors"
	"testing"

	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
)

type templateStoreStub struct {
	items  []model.ComposeTemplate
	called bool
}

func (s *templateStoreStub) ListActiveTemplates() ([]model.ComposeTemplate, error) {
	s.called = true
	return s.items, nil
}

func TestTemplateCatalogRequiresAnEnabledUser(t *testing.T) {
	store := &templateStoreStub{}
	_, err := NewTemplateService(store).List(identityservice.PublicUser{})
	if !errors.Is(err, ErrTemplateForbidden) {
		t.Fatalf("List() error = %v, want ErrTemplateForbidden", err)
	}
	if store.called {
		t.Fatal("disabled user must not query templates")
	}
}

func TestTemplateCatalogReturnsEmptyListBeforePublication(t *testing.T) {
	store := &templateStoreStub{}
	items, err := NewTemplateService(store).List(identityservice.PublicUser{Status: identityservice.UserStatusEnabled})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if !store.called || items == nil || len(items) != 0 {
		t.Fatalf("List() = %#v, queried=%t; want a queried empty list", items, store.called)
	}
}
