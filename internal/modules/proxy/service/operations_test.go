package service

import (
	"context"
	"reflect"
	"testing"

	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/model"
)

func TestProxyServiceExposesPackageLevelOperations(t *testing.T) {
	for _, item := range []struct {
		name string
		got  any
	}{
		{name: "BulkParse", got: BulkParse},
		{name: "Import", got: Import},
		{name: "LookupConnectionKeys", got: LookupConnectionKeys},
		{name: "List", got: List},
		{name: "ParseAddress", got: ParseAddress},
		{name: "Get", got: Get},
		{name: "Create", got: Create},
		{name: "Update", got: Update},
		{name: "UpdateStatus", got: UpdateStatus},
		{name: "Delete", got: Delete},
		{name: "CheckProxy", got: CheckProxy},
		{name: "CheckQuota", got: CheckQuota},
		{name: "CheckAssignable", got: CheckAssignable},
		{name: "SetMaxProfileCount", got: SetMaxProfileCount},
		{name: "RecordCheckResult", got: RecordCheckResult},
		{name: "WithAssignmentLock", got: WithAssignmentLock},
	} {
		if reflect.TypeOf(item.got).Kind() != reflect.Func {
			t.Fatalf("%s is not a package-level function", item.name)
		}
	}
}

func TestProxyCheckRunsInTheCloudInsteadOfTheAgent(t *testing.T) {
	// 提取和变更仍在运营桌面执行，但连通性检测是 Cloud 能力：签名不能要求
	// Agent 上下文或本机拼出的 ProxyCheckInput。
	var _ func(string) (model.ProxyConfig, dto.ProxyCheckResult, error) = CheckProxy
	var _ func(context.Context, dto.ProxyExtractionInput) (dto.ProxyExtractionResult, error) = ExtractProxy
	var _ func(context.Context, dto.ProxyMutationInput) (dto.ProxyMutationResult, error) = MutateProxy
}
