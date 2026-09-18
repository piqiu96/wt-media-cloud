package service

import (
	"reflect"
	"testing"
)

func TestCloudAgentServiceExposesPackageLevelOperations(t *testing.T) {
	tests := []struct {
		name string
		got  any
	}{
		{name: "RegisterAgent", got: RegisterAgent},
		{name: "Heartbeat", got: Heartbeat},
		{name: "GetAgent", got: GetAgent},
		{name: "CreateTask", got: CreateTask},
		{name: "ClaimTask", got: ClaimTask},
		{name: "ReportTask", got: ReportTask},
		{name: "CancelTask", got: CancelTask},
	}
	for _, test := range tests {
		if reflect.TypeOf(test.got).Kind() != reflect.Func {
			t.Fatalf("%s is not a function", test.name)
		}
	}
}
