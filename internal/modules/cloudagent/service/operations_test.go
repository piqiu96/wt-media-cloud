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
		{name: "CreateTask", got: CreateTask},
		{name: "ClaimTask", got: ClaimTask},
		{name: "ReportTask", got: ReportTask},
		{name: "ActivityTask", got: ActivityTask},
		{name: "CancelTask", got: CancelTask},
		{name: "RetryTask", got: RetryTask},
	}
	for _, test := range tests {
		if reflect.TypeOf(test.got).Kind() != reflect.Func {
			t.Fatalf("%s is not a function", test.name)
		}
	}
}
