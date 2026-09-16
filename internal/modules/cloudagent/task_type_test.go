package cloudagent

import "testing"

func TestDiscoveryIsNotACloudAgentTaskType(t *testing.T) {
	if _, ok := ParseTaskType("discovery_task"); ok {
		t.Fatal("M3 discovery must not be claimable by Cloud Agent")
	}
}
