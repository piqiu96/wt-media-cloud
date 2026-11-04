package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/executor"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
)

type verifyingOutputLink struct {
	exists bool
	err    error
	signs  int
}

func (l *verifyingOutputLink) ObjectExists(context.Context, string) (bool, error) {
	return l.exists, l.err
}
func (l *verifyingOutputLink) PresignObjectURL(context.Context, string) (string, error) {
	l.signs++
	return "signed", nil
}
func TestOutputObjectMustBeVerifiedBeforeGrant(t *testing.T) {
	for _, tc := range []struct {
		name    string
		exists  bool
		failure error
		want    error
	}{{"missing", false, nil, ErrComposeOutputUnavailable}, {"storage error", false, errors.New("secret storage endpoint"), ErrComposeOutputVerificationFailed}} {
		t.Run(tc.name, func(t *testing.T) {
			link := &verifyingOutputLink{exists: tc.exists, err: tc.failure}
			s := NewComposeOutputService(&outputMemoryStore{outputs: []model.CompositeOutput{outputFixture(1)}}, &outputTraceFakes{}, &outputTraceFakes{}, &outputTraceFakes{}, &outputTaskFake{}, link, &stubTransfers{})
			_, err := s.VideoURL(context.Background(), identity.PublicUser{ID: 1, Role: identity.RoleAdmin, Status: identity.UserStatusEnabled}, 1)
			if !errors.Is(err, tc.want) || link.signs != 0 {
				t.Fatalf("err=%v signs=%d", err, link.signs)
			}
		})
	}
}
func TestUnknownOutputOwnerHiddenFromOrdinaryCreator(t *testing.T) {
	o := outputFixture(1)
	o.OperatorID = nil
	team := identity.TeamID(7)
	actor := identity.PublicUser{ID: 9, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, CategoryIDs: []identity.CategoryID{1}}
	if canReadComposeOutput(actor, o) {
		t.Fatal("unowned asset granted to creator")
	}
}

type outputPagingFake struct {
	outputMemoryStore
	calls    int
	filter   repository.ComposeOutputFilter
	refs     []repository.ComposeOutputObjectRef
	template model.ComposeTemplate
}

func (f *outputPagingFake) Page(_ context.Context, filter repository.ComposeOutputFilter, p, n int) (repository.ComposeOutputPage, error) {
	f.calls++
	f.filter = filter
	return repository.ComposeOutputPage{Items: append([]model.CompositeOutput{}, f.outputs...), Total: int64(len(f.outputs)), Page: p, PageSize: n}, nil
}
func (f *outputPagingFake) MaterialPage(_ context.Context, filter repository.ComposeOutputFilter, p, n int) (repository.ComposeOutputMaterialPage, error) {
	f.calls++
	f.filter = filter
	return repository.ComposeOutputMaterialPage{Items: []repository.ComposeOutputMaterial{}, Page: p, PageSize: n}, nil
}
func (f *outputPagingFake) ObjectRefs(_ context.Context, filter repository.ComposeOutputFilter) ([]repository.ComposeOutputObjectRef, error) {
	f.filter = filter
	return f.refs, nil
}
func (f *outputPagingFake) OriginalTemplate(string, uint32) (model.ComposeTemplate, bool, error) {
	return f.template, f.template.Code != "", nil
}
func TestOutputEmptyCategoriesShortCircuitBeforeRepository(t *testing.T) {
	team := identity.TeamID(7)
	actor := identity.PublicUser{ID: 9, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team}
	repo := &outputPagingFake{}
	s := NewComposeOutputService(repo, &outputTraceFakes{}, &outputTraceFakes{}, &outputTraceFakes{}, &outputTaskFake{}, &verifyingOutputLink{}, &stubTransfers{})
	page, err := s.Page(context.Background(), actor, ListComposeOutputsInput{})
	if err != nil || len(page.Items) != 0 || repo.calls != 0 {
		t.Fatalf("page=%+v err=%v calls=%d", page, err, repo.calls)
	}
	aggregate, err := s.MaterialPage(context.Background(), actor, ListComposeOutputsInput{})
	if err != nil || len(aggregate.Items) != 0 || repo.calls != 0 {
		t.Fatalf("aggregate=%+v err=%v calls=%d", aggregate, err, repo.calls)
	}
}
func TestOutputTraceUsesOriginalSnapshotAndSafeOriginalSchema(t *testing.T) {
	o := outputFixture(1)
	repo := &outputPagingFake{outputMemoryStore: outputMemoryStore{outputs: []model.CompositeOutput{o}}, template: model.ComposeTemplate{Code: "basic_cut", Version: 1, Schema: json.RawMessage(`{"type":"object","properties":{"guide_video_url":{"type":"string","default":"https://secret.example/guide","editable":false},"end_seconds":{"type":"number","editable":true}}}`)}}
	traces := &outputTraceFakes{strategy: model.ComposeStrategy{ID: 3, Name: "策略", Config: json.RawMessage(`{"end_seconds":999,"guide_video_url":"https://current.secret"}`)}}
	task := &outputTaskFake{task: model.ComposeTask{ID: 55, Status: model.ComposeTaskSuccess, ConfigSnapshot: json.RawMessage(`{"end_seconds":2,"guide_video_url":"https://original.secret"}`), SettingKey: "original", SettingName: "原组", PlatformsJSON: json.RawMessage(`["baidu"]`)}}
	s := NewComposeOutputService(repo, traces, traces, traces, task, &verifyingOutputLink{exists: true}, &stubTransfers{})
	detail, err := s.Get(identity.PublicUser{ID: 1, Role: identity.RoleAdmin, Status: identity.UserStatusEnabled}, 1)
	if err != nil {
		t.Fatal(err)
	}
	wire, _ := json.Marshal(detail)
	if bytes.Contains(wire, []byte("secret")) || bytes.Contains(wire, []byte("999")) {
		t.Fatalf("mutable or private data leaked: %s", wire)
	}
	if !bytes.Contains(wire, []byte(`"end_seconds":2`)) || detail.Trace.ComposeTask.SettingKey != "original" || detail.Trace.Publishing.Available {
		t.Fatalf("detail=%s", wire)
	}
}

type concurrentOutputLink struct {
	active atomic.Int32
	peak   atomic.Int32
	fail   bool
}

func (l *concurrentOutputLink) ObjectExists(ctx context.Context, _ string) (bool, error) {
	n := l.active.Add(1)
	defer l.active.Add(-1)
	for {
		p := l.peak.Load()
		if n <= p || l.peak.CompareAndSwap(p, n) {
			break
		}
	}
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case <-time.After(time.Millisecond):
	}
	if l.fail {
		return false, errors.New("private endpoint failure")
	}
	return true, nil
}
func (l *concurrentOutputLink) PresignObjectURL(context.Context, string) (string, error) {
	return "", nil
}
func TestOutputVideoFilterBoundedAndFailureNeverReturnsFalseTotal(t *testing.T) {
	for _, fail := range []bool{false, true} {
		repo := &outputPagingFake{}
		for i := int64(1); i <= 40; i++ {
			repo.refs = append(repo.refs, repository.ComposeOutputObjectRef{ID: i, ObjectKey: "key"})
		}
		link := &concurrentOutputLink{fail: fail}
		s := NewComposeOutputService(repo, &outputTraceFakes{}, &outputTraceFakes{}, &outputTraceFakes{}, &outputTaskFake{}, link, &stubTransfers{})
		_, err := s.Page(context.Background(), identity.PublicUser{Role: identity.RoleAdmin, Status: identity.UserStatusEnabled}, ListComposeOutputsInput{VideoStatus: "ready"})
		if link.peak.Load() > 8 {
			t.Fatalf("unbounded parallel checks: %d", link.peak.Load())
		}
		if fail {
			if !errors.Is(err, ErrComposeOutputVerificationFailed) || repo.calls != 0 {
				t.Fatalf("failed verification reached page: err=%v calls=%d", err, repo.calls)
			}
		} else if err != nil || !repo.filter.RestrictIDs || len(repo.filter.OutputIDs) != 40 {
			t.Fatalf("err=%v filter=%+v", err, repo.filter)
		}
	}
}

type availableOutputTrace struct{ outputTraceFakes }

func (f *availableOutputTrace) OutputInputAvailable(model.Material) (bool, error) { return true, nil }
func TestRemakeRequiresExecutableOriginalParamsAndIgnoresMissingOutputObject(t *testing.T) {
	team := identity.TeamID(7)
	owner := identity.PublicUser{ID: 9, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, CategoryIDs: []identity.CategoryID{1}}
	definition, err := executor.DeployedDefinition("basic_cut", 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		params string
		remake bool
	}{{`{"start_seconds":0,"end_seconds":2}`, true}, {`{}`, false}} {
		o := outputFixture(1)
		task := model.ComposeTask{ID: o.TaskID, TeamID: o.TeamID, CategoryID: o.CategoryID, OperatorID: o.OperatorID, MaterialID: o.MaterialID, StrategyID: o.StrategyID, TemplateCode: o.TemplateCode, TemplateVersion: o.TemplateVersion, SettingKey: "original", SettingName: "原组", PlatformsJSON: json.RawMessage(`["bilibili"]`), ConfigSnapshot: json.RawMessage(tc.params), Status: model.ComposeTaskSuccess}
		repo := &outputPagingFake{outputMemoryStore: outputMemoryStore{outputs: []model.CompositeOutput{o}}, template: model.ComposeTemplate{Code: "basic_cut", Version: 1, Status: "active", ExecutorID: definition.ExecutorID, Schema: definition.Schema}}
		category := identity.CategoryID(1)
		traces := &availableOutputTrace{outputTraceFakes{material: model.Material{ID: 42, TeamID: 7, CategoryID: &category, Status: model.MaterialAvailable, SourceURL: "https://source"}, strategy: model.ComposeStrategy{ID: 3, TeamID: 7, CategoryID: 1, OperatorID: ptrInt64(9), Status: model.ComposeStrategyEnabled}}}
		s := NewComposeOutputService(repo, traces, traces, traces, &outputTaskFake{task: task}, &verifyingOutputLink{exists: false}, &stubTransfers{})
		s.operators = func() ([]identity.PublicUser, error) { return []identity.PublicUser{owner}, nil }
		detail, err := s.Get(owner, 1)
		if err != nil {
			t.Fatal(err)
		}
		if detail.Output.Actions.Remake != tc.remake || detail.Output.Actions.Download || detail.Output.Video.Status != "failed" {
			t.Fatalf("params=%s actions=%+v video=%+v", tc.params, detail.Output.Actions, detail.Output.Video)
		}
	}
}

func TestOutputDownloadVerificationFailureNeverQueuesTransfer(t *testing.T) {
	for _, link := range []*verifyingOutputLink{{exists: false}, {err: errors.New("private storage failure")}} {
		transfers := &stubTransfers{}
		s := NewComposeOutputService(&outputMemoryStore{outputs: []model.CompositeOutput{outputFixture(1)}}, &outputTraceFakes{}, &outputTraceFakes{}, &outputTraceFakes{}, &outputTaskFake{}, link, transfers)
		team := identity.TeamID(7)
		_, err := s.CreateOutputDownload(identity.PublicUser{ID: 9, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, CategoryIDs: []identity.CategoryID{1}}, 1)
		if err == nil || transfers.count != 0 {
			t.Fatalf("err=%v transfers=%d", err, transfers.count)
		}
	}
}

func TestOutputPagePreservesPersistedAssetFileName(t *testing.T) {
	output := outputFixture(1)
	output.Name = "素材 · 短版"
	output.FileName = "素材 · 短版-1.mp4"
	repo := &outputPagingFake{outputMemoryStore: outputMemoryStore{outputs: []model.CompositeOutput{output}}}
	svc := NewComposeOutputService(repo, &outputTraceFakes{}, &outputTraceFakes{}, &outputTraceFakes{}, &outputTaskFake{}, &verifyingOutputLink{exists: true}, &stubTransfers{})
	page, err := svc.Page(context.Background(), identity.PublicUser{ID: 1, Role: identity.RoleAdmin, Status: identity.UserStatusEnabled}, ListComposeOutputsInput{})
	if err != nil || len(page.Items) != 1 || page.Items[0].FileName != output.FileName {
		t.Fatalf("page=%+v err=%v", page, err)
	}
}
