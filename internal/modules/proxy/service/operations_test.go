package service

import (
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
		{name: "UpdateEnabled", got: UpdateEnabled},
		{name: "Delete", got: Delete},
		{name: "CheckProxy", got: CheckProxy},
		{name: "CheckQuota", got: CheckQuota},
		{name: "CheckAssignable", got: CheckAssignable},
		{name: "SetMaxProfiles", got: SetMaxProfiles},
		{name: "RecordCheckResult", got: RecordCheckResult},
		{name: "WithAssignmentLock", got: WithAssignmentLock},
	} {
		if reflect.TypeOf(item.got).Kind() != reflect.Func {
			t.Fatalf("%s is not a package-level function", item.name)
		}
	}
}

func TestProxyCheckRunsInTheCloudInsteadOfTheAgent(t *testing.T) {
	// 动态提取和 BitBrowser 写入都由 Desktop/Agent 执行；Cloud 只负责连通性
	// 检测和正式状态结算。
	var _ func(string) (model.Proxy, dto.ProxyCheckResult, error) = CheckProxy
}
