package model

import (
	"reflect"
	"testing"
)

func TestCloudAgentModelTypesAreDeclaredInModelPackage(t *testing.T) {
	tests := []struct {
		name string
		got  reflect.Type
	}{
		{name: "AgentNode", got: reflect.TypeFor[AgentNode]()},
		{name: "Task", got: reflect.TypeFor[Task]()},
		{name: "Compatibility", got: reflect.TypeFor[Compatibility]()},
	}
	for _, test := range tests {
		if got, want := test.got.PkgPath(), "github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/model"; got != want {
			t.Fatalf("%s PkgPath = %q, want %q", test.name, got, want)
		}
	}
}
