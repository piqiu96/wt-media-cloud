package cloudagent

import "strconv"

const (
	APIName                      = "cloud-agent"
	MajorVersion                 = "v1"
	ContractRevision             = "2026.07.14.4"
	MinimumAgentContractRevision = "2026.07.14.4"
)

type Compatibility struct {
	API                          string   `json:"api"`
	MajorVersion                 string   `json:"major_version"`
	ContractRevision             string   `json:"contract_revision"`
	MinimumAgentContractRevision string   `json:"minimum_agent_contract_revision"`
	CompatibleAgentMajorVersions []string `json:"compatible_agent_major_versions"`
	Status                       string   `json:"status"`
}

func CurrentCompatibility() Compatibility {
	return Compatibility{
		API:                          APIName,
		MajorVersion:                 MajorVersion,
		ContractRevision:             ContractRevision,
		MinimumAgentContractRevision: MinimumAgentContractRevision,
		CompatibleAgentMajorVersions: []string{MajorVersion},
		Status:                       "compatible",
	}
}

func IsAgentCompatible(majorVersion string, contractRevision string) bool {
	if majorVersion != MajorVersion {
		return false
	}
	return compareRevision(contractRevision, MinimumAgentContractRevision) >= 0
}

func compareRevision(left string, right string) int {
	leftParts, leftOK := parseRevision(left)
	rightParts, rightOK := parseRevision(right)
	if !leftOK || !rightOK {
		return -1
	}
	for i := range leftParts {
		if leftParts[i] > rightParts[i] {
			return 1
		}
		if leftParts[i] < rightParts[i] {
			return -1
		}
	}
	return 0
}

func parseRevision(value string) ([4]int, bool) {
	var result [4]int
	start := 0
	part := 0
	for i := 0; i <= len(value); i++ {
		if i != len(value) && value[i] != '.' {
			continue
		}
		if part >= len(result) || start == i {
			return result, false
		}
		n, err := strconv.Atoi(value[start:i])
		if err != nil {
			return result, false
		}
		result[part] = n
		part++
		start = i + 1
	}
	return result, part == len(result)
}
