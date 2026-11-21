package service

import (
	"context"
	"errors"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	production "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
	"strings"
	"testing"
)

func generationInput() GenerationInput {
	return GenerationInput{RowKey: "row", InputRevision: "1", Platform: "baidu", CategoryID: 1, CompositeOutputID: 42, Fields: []string{"description", "topics"}, FieldRevisions: map[string]string{"description": "2", "topics": "0"}, Content: map[string]string{"description": "人工初稿", "topics": ""}, Instruction: "简洁"}
}
func TestGenerationUsesAuthorizedSourceAndNeverCreatesOrConfirmsTask(t *testing.T) {
	s, a, _ := fixture()
	baseOutput := s.Output
	s.Output = func(a identity.PublicUser, id int64) (production.ComposeOutputDetail, error) {
		v, e := baseOutput(a, id)
		v.Output.MaterialTitle = "真实素材标题"
		return v, e
	}
	in := generationInput()
	run, err := s.PreparePublicationGeneration(a, in)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	s.GenerateJSON = func(ctx context.Context, system, user string) (string, error) {
		calls++
		if !strings.Contains(user, "真实素材标题") || !strings.Contains(user, "人工初稿") || !strings.Contains(system, "JSON") {
			t.Fatal("missing authorized basis")
		}
		return `{"description":"新的建议","topics":"#实战"}`, nil
	}
	result, err := run(context.Background())
	if err != nil || result.Content["description"] != "新的建议" || result.FieldRevisions["description"] != "2" || calls != 1 || len(s.Repo.(*memoryTasks).items) != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if in.Content["description"] != "人工初稿" {
		t.Fatal("generation overwrote input")
	}
}
func TestGenerationRefusesInvalidOrUnauthorizedBasisBeforeProvider(t *testing.T) {
	for _, modify := range []func(*GenerationInput){func(i *GenerationInput) { i.Fields = []string{"declaration"} }, func(i *GenerationInput) { i.FieldRevisions = nil }, func(i *GenerationInput) { i.Platform = "bilibili" }, func(i *GenerationInput) { i.InputRevision = "-1" }, func(i *GenerationInput) { i.CategoryID = 2 }, func(i *GenerationInput) { i.Fields = []string{"description", "description"} }} {
		s, a, _ := fixture()
		in := generationInput()
		modify(&in)
		s.GenerateJSON = func(context.Context, string, string) (string, error) {
			t.Fatal("provider called for invalid basis")
			return "", nil
		}
		if _, err := s.PreparePublicationGeneration(a, in); err == nil {
			t.Fatal("invalid generation accepted")
		}
	}
	s, a, _ := fixture()
	s.Output = func(identity.PublicUser, int64) (production.ComposeOutputDetail, error) {
		return production.ComposeOutputDetail{}, production.ErrComposeOutputForbidden
	}
	if _, err := s.PreparePublicationGeneration(a, generationInput()); !errors.Is(err, production.ErrComposeOutputForbidden) {
		t.Fatalf("scope=%v", err)
	}
}
func TestGenerationRequiresAllSelectedValidFieldsAndNoExtraOutput(t *testing.T) {
	for _, raw := range []string{`{"description":"部分"}`, `{"description":"有效","topics":null}`, `{"description":"有效","topics":"","declaration":"原创"}`, `{"description":"有效","topics":3}`, `{"description":"有效","topics":"` + strings.Repeat("超", 201) + `"}`, `{"description":" ","topics":""}`, `{} {}`} {
		s, a, _ := fixture()
		s.GenerateJSON = func(context.Context, string, string) (string, error) { return raw, nil }
		run, err := s.PreparePublicationGeneration(a, generationInput())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := run(context.Background()); err == nil {
			t.Fatalf("invalid output accepted: %s", raw)
		}
	}
}
