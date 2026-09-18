package dto

import (
	"reflect"
	"testing"
)

func TestProfileBindingDTOTypesAreDeclaredInDTOPackage(t *testing.T) {
	for _, item := range []struct {
		name string
		got  reflect.Type
	}{
		{name: "ProfileInput", got: reflect.TypeFor[ProfileInput]()},
		{name: "SnapshotInput", got: reflect.TypeFor[SnapshotInput]()},
		{name: "MainIdentityInput", got: reflect.TypeFor[MainIdentityInput]()},
	} {
		if got, want := item.got.PkgPath(), "github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/dto"; got != want {
			t.Fatalf("%s PkgPath = %q, want %q", item.name, got, want)
		}
	}
}
