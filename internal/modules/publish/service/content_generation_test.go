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

func TestGenerationRejectsCombinedBaiduTextBeforeAdoption(t *testing.T) {
	s, a, _ := fixture()
	s.GenerateJSON = func(context.Context, string, string) (string, error) {
		return `{"description":"` + strings.Repeat("字", 40) + `","topics":"#` + strings.Repeat("话", 10) + `"}`, nil
	}
	run, err := s.PreparePublicationGeneration(a, generationInput())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = run(context.Background()); err == nil {
		t.Fatal("combined text exceeding 50 characters accepted")
	}
}

func TestGenerationCanShortenOverlongHumanDraft(t *testing.T) {
	s, a, _ := fixture()
	in := generationInput()
	in.Content["description"] = strings.Repeat("长", 100)
	s.GenerateJSON = func(context.Context, string, string) (string, error) {
		return `{"description":"精简标题","topics":"#实战"}`, nil
	}
	run, err := s.PreparePublicationGeneration(a, in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len([]rune(in.Content["description"])) != 100 {
		t.Fatal("original human text changed")
	}
}

func TestCreateRefusesCombinedTextOverLimitWithoutTruncation(t *testing.T) {
	s, a, in := fixture()
	in.Content.Description = strings.Repeat("字", 40)
	in.Content.Topics = "#" + strings.Repeat("话", 10)
	if _, err := s.Create(context.Background(), a, in); err == nil {
		t.Fatal("overlong platform text accepted")
	}
	if len(s.Repo.(*memoryTasks).items) != 0 {
		t.Fatal("invalid text occupied output")
	}
	in.Content.Topics = "#" + strings.Repeat("话", 8)
	if _, err := s.Create(context.Background(), a, in); err != nil {
		t.Fatal(err)
	}
}

func TestGenerationRejectsSeniorOutsidePublicationCategoryBeforeProviders(t *testing.T) {
	svc, actor, _ := fixture()
	actor.Role = identity.RoleSeniorOperator
	actor.CategoryIDs = []identity.CategoryID{2}
	in := generationInput()
	sourceCalls, modelCalls := 0, 0
	underlying := svc.Output
	svc.Output = func(a identity.PublicUser, id int64) (production.ComposeOutputDetail, error) {
		sourceCalls++
		return underlying(a, id)
	}
	svc.GenerateJSON = func(context.Context, string, string) (string, error) {
		modelCalls++
		return `{"description":"scope denied","topics":""}`, nil
	}
	if err := ValidateBatchGeneration(actor, in); !errors.Is(err, ErrNotFound) {
		t.Errorf("batch preflight scope = %v", err)
	}
	run, err := svc.PreparePublicationGeneration(actor, in)
	if run != nil {
		_, _ = run(context.Background())
	}
	if !errors.Is(err, ErrNotFound) || sourceCalls != 0 || modelCalls != 0 {
		t.Fatalf("unauthorized generation: result=%v source_calls=%d model_calls=%d", err, sourceCalls, modelCalls)
	}
	// Spoofing an authorized category still cannot send a title from a protected category.
	in.CategoryID = 2
	run, err = svc.PreparePublicationGeneration(actor, in)
	if run != nil {
		_, _ = run(context.Background())
	}
	if !errors.Is(err, ErrNotFound) || sourceCalls != 1 || modelCalls != 0 {
		t.Fatalf("mismatched source category: result=%v source_calls=%d model_calls=%d", err, sourceCalls, modelCalls)
	}
}

func TestGenerationUsesDistinctPlatformStyleAndRejectsInventedGameContext(t *testing.T) {
	for _, platform := range []string{"baidu", "bilibili"} {
		t.Run(platform, func(t *testing.T) {
			s, a, _ := fixture()
			s.PlatformSettings.BilibiliEnabled = true
			in := generationInput()
			in.Platform = platform
			in.Fields = []string{"description"}
			in.FieldRevisions = map[string]string{"description": "1"}
			in.Content = map[string]string{"description": ""}
			s.GenerateJSON = func(_ context.Context, system, user string) (string, error) {
				for _, requirement := range []string{"游戏名称", "不编造", "赛季", "手游", "引流", "插入广告"} {
					if !strings.Contains(system, requirement) {
						t.Errorf("%s prompt lacks game context constraint %s", platform, requirement)
					}
				}
				if platform == "bilibili" && (!strings.Contains(system, "玩家口吻") || !strings.Contains(system, "不重复标题")) {
					t.Error("missing Bilibili title/description style")
				}
				if platform == "baidu" && (!strings.Contains(system, "信息明确") || !strings.Contains(system, "事件")) {
					t.Error("missing Baidu informative style")
				}
				return `{"description":"三角洲行动实战片段"}`, nil
			}
			run, err := s.PreparePublicationGeneration(a, in)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = run(context.Background()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestGenerationUsesMaterialBodyWithoutInheritedHashtagCampaigns(t *testing.T) {
	s, a, _ := fixture()
	source := s.Output
	s.Output = func(a identity.PublicUser, id int64) (production.ComposeOutputDetail, error) {
		d, e := source(a, id)
		d.Output.MaterialTitle = "三角洲跳拉操作集锦 #双子星 #三角洲S12 #抖音精选app"
		return d, e
	}
	s.GenerateJSON = func(_ context.Context, system, user string) (string, error) {
		if !strings.Contains(user, "三角洲跳拉操作集锦") {
			t.Error("lost material body")
		}
		for _, tag := range []string{"双子星", "三角洲S12", "抖音精选app"} {
			if strings.Contains(user, tag) {
				t.Errorf("inherited unverified campaign %s", tag)
			}
		}
		return `{"description":"三角洲跳拉操作集锦","topics":""}`, nil
	}
	run, e := s.PreparePublicationGeneration(a, generationInput())
	if e != nil {
		t.Fatal(e)
	}
	if _, e = run(context.Background()); e != nil {
		t.Fatal(e)
	}
}
