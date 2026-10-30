package cloudagent

import (
	"context"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
)

type compatibility struct {
	API                          string   `json:"api"`
	MajorVersion                 string   `json:"major_version"`
	ContractRevision             string   `json:"contract_revision"`
	MinimumAgentContractRevision string   `json:"minimum_agent_contract_revision"`
	CompatibleAgentMajorVersions []string `json:"compatible_agent_major_versions"`
	Status                       string   `json:"status"`
}

func Compatibility(ctx context.Context, c *hertzapp.RequestContext) {
	api.Success(c, compatibility{
		API:                          "cloud-agent",
		MajorVersion:                 "v1",
		ContractRevision:             "2026.10.29.2",
		MinimumAgentContractRevision: "2026.10.23.1",
		CompatibleAgentMajorVersions: []string{"v1"},
		Status:                       "compatible",
	})
}
