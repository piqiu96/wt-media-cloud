package model

import (
	"reflect"
	"testing"
)

func TestRuntimeBindingModelTypesAreDeclaredInModelPackage(t *testing.T) {
	for _, item := range []struct {
		name string
		got  reflect.Type
	}{
		{name: "BindingTicket", got: reflect.TypeFor[BindingTicket]()},
		{name: "AgentNode", got: reflect.TypeFor[AgentNode]()},
		{name: "DependencyFact", got: reflect.TypeFor[DependencyFact]()},
		{name: "DiskFact", got: reflect.TypeFor[DiskFact]()},
	} {
		if got, want := item.got.PkgPath(), "github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding/model"; got != want {
			t.Fatalf("%s PkgPath = %q, want %q", item.name, got, want)
		}
	}
}
