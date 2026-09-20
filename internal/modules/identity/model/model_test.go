package model

import (
	"reflect"
	"testing"
)

func TestIdentityModelTypesAreDeclaredInModelPackage(t *testing.T) {
	tests := []struct {
		name string
		got  reflect.Type
		want string
	}{
		{name: "User", got: reflect.TypeFor[User](), want: "github.com/wt-media/wt-media-cloud/internal/modules/identity/model"},
		{name: "PublicUser", got: reflect.TypeFor[PublicUser](), want: "github.com/wt-media/wt-media-cloud/internal/modules/identity/model"},
		{name: "Session", got: reflect.TypeFor[Session](), want: "github.com/wt-media/wt-media-cloud/internal/modules/identity/model"},
		{name: "AuditEvent", got: reflect.TypeFor[AuditEvent](), want: "github.com/wt-media/wt-media-cloud/internal/shared/identity"},
	}
	for _, test := range tests {
		if got := test.got.PkgPath(); got != test.want {
			t.Fatalf("%s PkgPath = %q, want %q", test.name, got, test.want)
		}
	}
}
