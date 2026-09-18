package model

import (
	"reflect"
	"testing"
)

func TestMediaAccountModelTypesAreDeclaredInModelPackage(t *testing.T) {
	for _, item := range []struct {
		name string
		got  reflect.Type
	}{
		{name: "Account", got: reflect.TypeFor[Account]()},
		{name: "AccountRecord", got: reflect.TypeFor[AccountRecord]()},
		{name: "AccountGroup", got: reflect.TypeFor[AccountGroup]()},
		{name: "AccountCheckItem", got: reflect.TypeFor[AccountCheckItem]()},
		{name: "Platform", got: reflect.TypeFor[Platform]()},
	} {
		if got, want := item.got.PkgPath(), "github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/model"; got != want {
			t.Fatalf("%s PkgPath = %q, want %q", item.name, got, want)
		}
	}
}
