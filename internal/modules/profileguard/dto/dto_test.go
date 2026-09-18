package dto

import (
	"reflect"
	"testing"
)

func TestProfileGuardDTOTypesAreDeclaredInDTOPackage(t *testing.T) {
	if got, want := reflect.TypeFor[PreflightOutcome]().PkgPath(), "github.com/wt-media/wt-media-cloud/internal/modules/profileguard/dto"; got != want {
		t.Fatalf("PreflightOutcome PkgPath = %q, want %q", got, want)
	}
}
