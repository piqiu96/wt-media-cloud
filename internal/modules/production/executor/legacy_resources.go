package executor

import (
	"encoding/json"
	"errors"
)

// Adapters keep the retiring templates on the same common execution entry
// during cutover. CHG125 removes these together with their old implementations.
func (p cutPlan) Resources(json.RawMessage) ([]ResourceRequest, error) { return nil, nil }
func (p cutPlan) Compile(input RenderInput) (RenderResult, error) {
	start, end := p.SegmentMS()
	if input.Source.Duration*1000 < float64(end) {
		return RenderResult{}, errors.New("source is shorter than selected segment")
	}
	return RenderResult{Arguments: p.FFmpegArguments(input.Source.Path, input.OutputPath), DurationMS: end - start}, nil
}
func (p guidePlan) Resources(json.RawMessage) ([]ResourceRequest, error) {
	return []ResourceRequest{{Name: "guide", URL: p.guideURL}}, nil
}
func (p guidePlan) Compile(input RenderInput) (RenderResult, error) {
	guide, ok := input.Resources["guide"]
	if !ok {
		return RenderResult{}, errors.New("guide video is required")
	}
	sourceMS, guideMS := int64(input.Source.Duration*1000), int64(guide.Duration*1000)
	duration, err := p.OutputDurationMS(sourceMS, guideMS)
	if err != nil {
		return RenderResult{}, err
	}
	args, err := p.FFmpegGuideArguments(input.Source.Path, guide.Path, input.OutputPath, sourceMS, guideMS, input.Source.Width, input.Source.Height, input.Source.HasAudio, guide.HasAudio)
	return RenderResult{Arguments: args, DurationMS: duration}, err
}
