package jobs

import (
	"context"
	"reflect"
	"testing"
)

func TestRunProxyExpiryUsesServiceBoundaryWithoutRawResources(t *testing.T) {
	typed := reflect.TypeFor[func(context.Context) error]()
	if reflect.TypeOf(RunProxyExpiry) != typed {
		t.Fatalf("RunProxyExpiry type = %T, want func(context.Context) error", RunProxyExpiry)
	}
}
