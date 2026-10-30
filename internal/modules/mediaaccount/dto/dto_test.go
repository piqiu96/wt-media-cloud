package dto

import (
	"reflect"
	"testing"
)

func TestMediaAccountDTOTypesAreDeclaredInDTOPackage(t *testing.T) {
	for _, item := range []struct {
		name string
		got  reflect.Type
	}{
		{name: "CreateAccountInput", got: reflect.TypeFor[CreateAccountInput]()},
		{name: "AccountCheckStartInput", got: reflect.TypeFor[AccountCheckStartInput]()},
		{name: "AccountCheckResultInput", got: reflect.TypeFor[AccountCheckResultInput]()},
		{name: "UpdateAccountInput", got: reflect.TypeFor[UpdateAccountInput]()},
		{name: "AccountFilter", got: reflect.TypeFor[AccountFilter]()},
	} {
		if got, want := item.got.PkgPath(), "github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/dto"; got != want {
			t.Fatalf("%s PkgPath = %q, want %q", item.name, got, want)
		}
	}
}
