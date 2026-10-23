package model

import "testing"

// 类型元数据必须与契约表一致；未知类型不降级成 noop。
func TestTaskTypeSpecsAndParsing(t *testing.T) {
	specs := map[TaskType]TaskTypeSpec{
		TaskTypeNoop:          {TimeoutSeconds: 300},
		TaskTypeCookieRead:    {TimeoutSeconds: 300},
		TaskTypeCookieWrite:   {TimeoutSeconds: 300, ExternalSideEffect: true},
		TaskTypeAccountCheck:  {TimeoutSeconds: 300},
		TaskTypeProfileCreate: {TimeoutSeconds: 300, ExternalSideEffect: true},
		TaskTypeProfileOpen:   {TimeoutSeconds: 120},
		TaskTypeProfileClose:  {TimeoutSeconds: 120},
		TaskTypeProfileUpdate: {TimeoutSeconds: 300, ExternalSideEffect: true},
		TaskTypeProxyCheck:    {TimeoutSeconds: 300},
		TaskTypeProxyMutation: {TimeoutSeconds: 300, ExternalSideEffect: true},
	}
	for taskType, want := range specs {
		if got := taskType.Spec(); got != want {
			t.Fatalf("%s spec = %#v, want %#v", taskType, got, want)
		}
		parsed, ok := ParseTaskTypeValue(taskType.String())
		if !ok || parsed != taskType {
			t.Fatalf("ParseTaskTypeValue(%q) = %v, %v; want %v, true", taskType.String(), parsed, ok, taskType)
		}
	}
	for _, value := range []string{"", "unknown", "discovery_task", "cookie_read"} {
		if _, ok := ParseTaskTypeValue(value); ok {
			t.Fatalf("ParseTaskTypeValue(%q) accepted an unknown type", value)
		}
	}
}
