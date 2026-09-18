package model

import (
	"reflect"
	"testing"
)

func TestProxyModelTypesAreDeclaredInModelPackage(t *testing.T) {
	for _, item := range []struct {
		name string
		got  reflect.Type
	}{
		{name: "ProxyConfig", got: reflect.TypeFor[ProxyConfig]()},
		{name: "ProxyProtocol", got: reflect.TypeFor[ProxyProtocol]()},
		{name: "BusinessStatus", got: reflect.TypeFor[BusinessStatus]()},
		{name: "ProxySourceType", got: reflect.TypeFor[ProxySourceType]()},
	} {
		if got, want := item.got.PkgPath(), "github.com/wt-media/wt-media-cloud/internal/modules/proxy/model"; got != want {
			t.Fatalf("%s PkgPath = %q, want %q", item.name, got, want)
		}
	}
}
