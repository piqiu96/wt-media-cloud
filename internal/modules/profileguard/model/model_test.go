package model

import (
	"reflect"
	"testing"
)

func TestProfileGuardModelTypesAreDeclaredInModelPackage(t *testing.T) {
	for _, item := range []struct {
		name string
		got  reflect.Type
	}{
		{name: "SensitiveTask", got: reflect.TypeFor[SensitiveTask]()},
		{name: "Permit", got: reflect.TypeFor[Permit]()},
	} {
		if got, want := item.got.PkgPath(), "github.com/wt-media/wt-media-cloud/internal/modules/profileguard/model"; got != want {
			t.Fatalf("%s PkgPath = %q, want %q", item.name, got, want)
		}
	}
}
