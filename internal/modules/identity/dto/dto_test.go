package dto

import (
	"reflect"
	"testing"
)

func TestIdentityDTOTypesAreDeclaredInDTOPackage(t *testing.T) {
	tests := []struct {
		name string
		got  reflect.Type
	}{
		{name: "CreateUserInput", got: reflect.TypeFor[CreateUserInput]()},
		{name: "LoginOptions", got: reflect.TypeFor[LoginOptions]()},
		{name: "LoginResult", got: reflect.TypeFor[LoginResult]()},
		{name: "AuthContext", got: reflect.TypeFor[AuthContext]()},
	}
	for _, test := range tests {
		if got, want := test.got.PkgPath(), "github.com/wt-media/wt-media-cloud/internal/modules/identity/dto"; got != want {
			t.Fatalf("%s PkgPath = %q, want %q", test.name, got, want)
		}
	}
}
