package service

import (
	"reflect"
	"testing"
)

func TestRuntimeBindingServiceExposesPackageLevelOperations(t *testing.T) {
	for _, item := range []struct {
		name string
		got  any
	}{
		{name: "IssueTicket", got: IssueTicket},
		{name: "RegisterLocal", got: RegisterLocal},
		{name: "CheckLocalTrust", got: CheckLocalTrust},
		{name: "ReportRuntime", got: ReportRuntime},
		{name: "AuthenticateNode", got: AuthenticateNode},
	} {
		if reflect.TypeOf(item.got).Kind() != reflect.Func {
			t.Fatalf("%s is not a package-level function", item.name)
		}
	}
}
