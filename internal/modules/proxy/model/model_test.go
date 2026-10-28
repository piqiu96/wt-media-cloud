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
		{name: "Proxy", got: reflect.TypeFor[Proxy]()},
		{name: "Protocol", got: reflect.TypeFor[Protocol]()},
		{name: "ProxySource", got: reflect.TypeFor[ProxySource]()},
	} {
		if got, want := item.got.PkgPath(), "github.com/wt-media/wt-media-cloud/internal/modules/proxy/model"; got != want {
			t.Fatalf("%s PkgPath = %q, want %q", item.name, got, want)
		}
	}
}
