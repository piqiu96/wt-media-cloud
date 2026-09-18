package dto

import (
	"reflect"
	"testing"
)

func TestCloudAgentDTOTypesAreDeclaredInDTOPackage(t *testing.T) {
	tests := []struct {
		name string
		got  reflect.Type
	}{
		{name: "RegisterAgentRequest", got: reflect.TypeFor[RegisterAgentRequest]()},
		{name: "HeartbeatRequest", got: reflect.TypeFor[HeartbeatRequest]()},
		{name: "CreateTaskRequest", got: reflect.TypeFor[CreateTaskRequest]()},
		{name: "ClaimTaskRequest", got: reflect.TypeFor[ClaimTaskRequest]()},
		{name: "ReportTaskRequest", got: reflect.TypeFor[ReportTaskRequest]()},
		{name: "CancelTaskRequest", got: reflect.TypeFor[CancelTaskRequest]()},
	}
	for _, test := range tests {
		if got, want := test.got.PkgPath(), "github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/dto"; got != want {
			t.Fatalf("%s PkgPath = %q, want %q", test.name, got, want)
		}
	}
}
