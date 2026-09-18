package service

import (
	"reflect"
	"testing"
)

func TestProfileBindingServiceExposesPackageLevelOperations(t *testing.T) {
	for _, item := range []struct {
		name string
		got  any
	}{
		{name: "SubmitScan", got: SubmitScan},
		{name: "GetScan", got: GetScan},
		{name: "ConfirmScan", got: ConfirmScan},
		{name: "ConfirmMainIdentity", got: ConfirmMainIdentity},
		{name: "ListProfiles", got: ListProfiles},
		{name: "AssignProfileOwner", got: AssignProfileOwner},
	} {
		if reflect.TypeOf(item.got).Kind() != reflect.Func {
			t.Fatalf("%s is not a package-level function", item.name)
		}
	}
}
