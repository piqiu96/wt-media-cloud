package dto

import (
	"encoding/json"
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
	} {
		if got, want := item.got.PkgPath(), "github.com/wt-media/wt-media-cloud/internal/modules/proxy/dto"; got != want {
			t.Fatalf("%s PkgPath = %q, want %q", item.name, got, want)
		}
	}
}

func TestProxyWriteInputCanExplicitlyClearAuthentication(t *testing.T) {
	var input CreateProxyInput
	if err := json.Unmarshal([]byte(`{"replace_credentials":true,"username":"","password":""}`), &input); err != nil {
		t.Fatal(err)
	}
	if !input.ReplaceCredentials || input.Username != "" || input.Password != "" {
		t.Fatalf("explicit credential replacement lost: %#v", input)
	}
}
