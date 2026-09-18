package model

import (
	"reflect"
	"testing"
)

func TestIdentityModelTypesAreDeclaredInModelPackage(t *testing.T) {
	tests := []struct {
		name string
		got  reflect.Type
	}{
		{name: "User", got: reflect.TypeFor[User]()},
		{name: "PublicUser", got: reflect.TypeFor[PublicUser]()},
		{name: "Session", got: reflect.TypeFor[Session]()},
		{name: "AuditEvent", got: reflect.TypeFor[AuditEvent]()},
	}
	for _, test := range tests {
		if got, want := test.got.PkgPath(), "github.com/wt-media/wt-media-cloud/internal/modules/identity/model"; got != want {
			t.Fatalf("%s PkgPath = %q, want %q", test.name, got, want)
		}
	}
}
