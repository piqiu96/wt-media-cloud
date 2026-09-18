package service

import (
	"context"
	"reflect"
	"testing"

	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/dto"
)

func TestProxyServiceExposesPackageLevelOperations(t *testing.T) {
	for _, item := range []struct {
		name string
		got  any
	}{
		{name: "BulkParse", got: BulkParse},
		{name: "BulkImport", got: BulkImport},
		{name: "List", got: List},
		{name: "ParseAddress", got: ParseAddress},
		{name: "Get", got: Get},
		{name: "Create", got: Create},
		{name: "Update", got: Update},
		{name: "UpdateStatus", got: UpdateStatus},
		{name: "Delete", got: Delete},
		{name: "TriggerCheck", got: TriggerCheck},
		{name: "CheckQuota", got: CheckQuota},
		{name: "CheckAssignable", got: CheckAssignable},
		{name: "SetMaxProfileCount", got: SetMaxProfileCount},
		{name: "RecordCheckResult", got: RecordCheckResult},
	} {
		if reflect.TypeOf(item.got).Kind() != reflect.Func {
			t.Fatalf("%s is not a package-level function", item.name)
		}
	}
}

func TestProxyServiceUsesTypedAgentClientBoundary(t *testing.T) {
	var _ func(context.Context, dto.ProxyCheckInput) (dto.ProxyCheckResult, error) = CheckProxy
	var _ func(context.Context, dto.ProxyExtractionInput) (dto.ProxyExtractionResult, error) = ExtractProxy
	var _ func(context.Context, dto.ProxyMutationInput) (dto.ProxyMutationResult, error) = MutateProxy
}
