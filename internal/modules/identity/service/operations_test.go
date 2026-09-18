package service

import (
	"reflect"
	"testing"
)

func TestIdentityServiceExposesPackageLevelOperations(t *testing.T) {
	tests := []struct {
		name string
		got  any
	}{
		{name: "BootstrapAdmin", got: BootstrapAdmin},
		{name: "CreateUser", got: CreateUser},
		{name: "Login", got: Login},
		{name: "Authenticate", got: Authenticate},
		{name: "GameReferences", got: GameReferences},
	}
	for _, test := range tests {
		if reflect.TypeOf(test.got).Kind() != reflect.Func {
			t.Fatalf("%s is not a function", test.name)
		}
	}
}
