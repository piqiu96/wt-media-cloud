package repository

import "github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding/model"

// RuntimeReport is the persistence projection of a local-node report.
type RuntimeReport struct {
	OperatingSystem  string
	CPUArchitecture  string
	AgentVersion     string
	PythonVersion    string
	FFmpeg           model.DependencyFact
	WorkdirStatus    string
	Disk             model.DiskFact
	BitBrowserStatus string
	MainUserID       string
	BitProfileIDs    []string
}
