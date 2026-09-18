package dto

import (
	"reflect"
	"testing"
)

func TestRuntimeBindingDTOTypesAreDeclaredInDTOPackage(t *testing.T) {
	for _, item := range []struct {
		name string
		got  reflect.Type
	}{
		{name: "BindingTicketGrant", got: reflect.TypeFor[BindingTicketGrant]()},
		{name: "RegisterLocalInput", got: reflect.TypeFor[RegisterLocalInput]()},
		{name: "Registration", got: reflect.TypeFor[Registration]()},
		{name: "RuntimeReport", got: reflect.TypeFor[RuntimeReport]()},
	} {
		if got, want := item.got.PkgPath(), "github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding/dto"; got != want {
			t.Fatalf("%s PkgPath = %q, want %q", item.name, got, want)
		}
	}
}
