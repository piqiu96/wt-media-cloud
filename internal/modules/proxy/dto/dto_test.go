package dto

import (
	"reflect"
	"testing"
)

func TestProxyDTOTypesAreDeclaredInDTOPackage(t *testing.T) {
	for _, item := range []struct {
		name string
		got  reflect.Type
	}{
		{name: "CreateProxyInput", got: reflect.TypeFor[CreateProxyInput]()},
		{name: "BulkImportRow", got: reflect.TypeFor[BulkImportRow]()},
		{name: "ProxyFilter", got: reflect.TypeFor[ProxyFilter]()},
		{name: "ProxyCheckInput", got: reflect.TypeFor[ProxyCheckInput]()},
		{name: "ProxyExtractionInput", got: reflect.TypeFor[ProxyExtractionInput]()},
		{name: "ProxyMutationInput", got: reflect.TypeFor[ProxyMutationInput]()},
	} {
		if got, want := item.got.PkgPath(), "github.com/wt-media/wt-media-cloud/internal/modules/proxy/dto"; got != want {
			t.Fatalf("%s PkgPath = %q, want %q", item.name, got, want)
		}
	}
}
