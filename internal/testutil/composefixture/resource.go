// Package composefixture supplies test stand-ins; runtime code must not import it.
package composefixture

import (
	"encoding/json"
	"errors"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/executor"
	"net/url"
)

const resourceSchema = `{"type":"object","properties":{"guide_video_url":{"type":"string","editable":false},"insert_seconds":{"type":"integer","minimum":0,"editable":true},"guide_duration_seconds":{"type":"integer","minimum":0,"editable":true},"max_duration_seconds":{"type":"integer","minimum":0,"editable":true}},"required":["guide_video_url","insert_seconds","guide_duration_seconds","max_duration_seconds"],"additionalProperties":false}`

type resourceExecutor struct{}
type resourcePlan struct{ address string }

func Resource() executor.Definition {
	return executor.Definition{Code: "fixture_resource", Version: 1, Schema: json.RawMessage(resourceSchema), ExecutorID: "fixture_resource_v1", Executor: resourceExecutor{}}
}
func (resourceExecutor) ValidateParams(raw json.RawMessage) (executor.Plan, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	if len(m) != 4 {
		return nil, errors.New("fixture params")
	}
	var u string
	if err := json.Unmarshal(m["guide_video_url"], &u); err != nil {
		return nil, err
	}
	v, err := url.Parse(u)
	if err != nil || v.Scheme != "https" || v.Hostname() == "" {
		return nil, errors.New("invalid URL")
	}
	for _, k := range []string{"insert_seconds", "guide_duration_seconds", "max_duration_seconds"} {
		n, err := integerParam(m[k], k)
		if err != nil || n < 0 {
			return nil, errors.New("invalid fixture number")
		}
	}
	return resourcePlan{u}, nil
}
func (p resourcePlan) Resources(json.RawMessage) ([]executor.ResourceRequest, error) {
	return []executor.ResourceRequest{{Name: "guide", URL: p.address}}, nil
}
func (p resourcePlan) Compile(in executor.RenderInput) (executor.RenderResult, error) {
	g, ok := in.Resources["guide"]
	if !ok {
		return executor.RenderResult{}, errors.New("missing test resource")
	}
	return executor.RenderResult{Arguments: []string{"-hide_banner", "-loglevel", "error", "-i", in.Source.Path, "-i", g.Path, "-filter_complex", "[0:v][0:a][1:v][1:a]concat=n=2:v=1:a=1[v][a]", "-map", "[v]", "-map", "[a]", "-c:v", "libx264", "-c:a", "aac", "-y", in.OutputPath}, DurationMS: int64((in.Source.Duration + g.Duration) * 1000)}, nil
}
