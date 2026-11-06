package executor

import "encoding/json"

// ResourcePlan is the common execution extension for templates that need
// additional media. The worker owns downloads, leases and publication; templates
// declare inputs and compile only their own validated rendering/timeline.
type ResourcePlan interface {
	Plan
	Resources(defaults json.RawMessage) ([]ResourceRequest, error)
	Compile(RenderInput) (RenderResult, error)
}
type DefaultsValidator interface{ ValidateDefaults(json.RawMessage) error }
type ResourceRequest struct {
	Name         string `json:"name"`
	URL          string `json:"url,omitempty"`
	PoolIndexKey string `json:"pool_index_key,omitempty"`
}
type MediaInput struct {
	Path     string  `json:"-"`
	Duration float64 `json:"duration"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	HasAudio bool    `json:"has_audio"`
}
type RenderInput struct {
	Source     MediaInput
	Resources  map[string]MediaInput
	Defaults   json.RawMessage
	Seed       int64
	OutputPath string
}
type RenderResult struct {
	Arguments    []string
	DurationMS   int64
	Timeline     json.RawMessage
	ToleranceMS  int64
	RequireAudio bool
}
