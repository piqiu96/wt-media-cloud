package service

import (
	"reflect"
	"testing"
)

func TestMediaAccountServiceExposesPackageLevelOperations(t *testing.T) {
	for _, item := range []struct {
		name string
		got  any
	}{
		{name: "CreateAccount", got: CreateAccount},
		{name: "IdentifyAccount", got: IdentifyAccount},
		{name: "StartAccountCheck", got: StartAccountCheck},
		{name: "ApplyAccountCheckResult", got: ApplyAccountCheckResult},
		{name: "StartCookieRead", got: StartCookieRead},
		{name: "ApplyCookieReadResult", got: ApplyCookieReadResult},
		{name: "ListAccounts", got: ListAccounts},
		{name: "CreateAccountGroup", got: CreateAccountGroup},
	} {
		if reflect.TypeOf(item.got).Kind() != reflect.Func {
			t.Fatalf("%s is not a package-level function", item.name)
		}
	}
}
