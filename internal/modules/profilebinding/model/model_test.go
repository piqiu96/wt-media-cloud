package model

import (
	"reflect"
	"testing"
)

func TestProfileBindingModelTypesAreDeclaredInModelPackage(t *testing.T) {
	for _, item := range []struct {
		name string
		got  reflect.Type
	}{
		{name: "BitAccountBinding", got: reflect.TypeFor[BitAccountBinding]()},
		{name: "BrowserProfile", got: reflect.TypeFor[BrowserProfile]()},
		{name: "ProfileDiff", got: reflect.TypeFor[ProfileDiff]()},
		{name: "ProfileScan", got: reflect.TypeFor[ProfileScan]()},
	} {
		if got, want := item.got.PkgPath(), "github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/model"; got != want {
			t.Fatalf("%s PkgPath = %q, want %q", item.name, got, want)
		}
	}
}
