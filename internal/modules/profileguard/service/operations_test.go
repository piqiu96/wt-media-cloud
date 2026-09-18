package service

import (
	"reflect"
	"testing"
)

func TestProfileGuardServiceExposesPackageLevelOperations(t *testing.T) {
	for _, item := range []struct {
		name string
		got  any
	}{
		{name: "Preflight", got: Preflight},
		{name: "Renew", got: Renew},
		{name: "Finish", got: Finish},
	} {
		if reflect.TypeOf(item.got).Kind() != reflect.Func {
			t.Fatalf("%s is not a package-level function", item.name)
		}
	}
}
