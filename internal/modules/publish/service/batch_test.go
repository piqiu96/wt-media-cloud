package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	filemodel "github.com/wt-media/wt-media-cloud/internal/modules/files/model"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	images "github.com/wt-media/wt-media-cloud/internal/modules/image/service"
	accounts "github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/service"
	outputmodel "github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	production "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
)

type batchOccupancyFixture struct {
	Repository
	err error
}

func (f batchOccupancyFixture) OccupiedOutputIDs(_ string, _ []int64) ([]int64, error) {
	return []int64{3}, f.err
}

func TestBatchAllocationHonorsSharedLimitBeforeAutomaticPairing(t *testing.T) {
	team := identity.TeamID(7)
	a := identity.PublicUser{ID: 9, TeamID: &team, CategoryIDs: []identity.CategoryID{1, 2}, Role: identity.RoleSeniorOperator, Status: identity.UserStatusEnabled}
	svc := Service{
		BatchCategory: func(id identity.CategoryID) (identity.Category, bool, error) {
			return identity.Category{ID: id, Enabled: true}, true, nil
		},
		Account: func(_ identity.PublicUser, id string) (accounts.Account, error) {
			owner := identity.UserID(9)
			if id == "11" {
				owner = 10
			}
			return accounts.Account{ID: id, UserID: owner, TeamID: &team, CategoryIDs: []identity.CategoryID{1, 2}, Platform: "baidu", Enabled: true, IdentificationStatus: accounts.IdentificationIdentified, PlatformLoginStatus: accounts.PlatformLoginNormal, PlatformAccountID: "fixture", BrowserProfileID: "1"}, nil
		},
		Output: func(_ identity.PublicUser, id int64) (production.ComposeOutputDetail, error) {
			category := identity.CategoryID(1)
			if id == 2 {
				category = 2
			}
			return production.ComposeOutputDetail{Output: outputmodel.CompositeOutput{ID: id, TeamID: 7, CategoryID: category, Platforms: []string{"baidu"}, Video: outputmodel.ComposeOutputVideo{Status: "ready"}, FileSizeBytes: 1, SHA256: strings.Repeat("b", 64)}}, nil
		},
		Repo: batchOccupancyFixture{},
	}
	in := BatchAllocationInput{
		Platform: "baidu",
		Targets:  []model.BatchTarget{{CategoryID: 1, TargetCount: 600}, {CategoryID: 2, TargetCount: 1}},
		BatchAllocation: model.BatchAllocation{
			Accounts:    []model.BatchAccountLimit{{MediaAccountID: 9, Limit: 1}, {MediaAccountID: 10, Limit: 1}},
			OutputIDs:   []string{"1", "2", "3"},
			ManualPairs: []model.BatchPair{{CategoryID: 2, MediaAccountID: 9, CompositeOutputID: 2}},
		},
	}
	out, err := svc.AllocateBatch(context.Background(), a, in)
	if err != nil || len(out.Config.Candidates) != 2 {
		t.Fatalf("allocation %+v %v", out, err)
	}
	first, second := out.Config.Candidates[0], out.Config.Candidates[1]
	if first.CategoryID != 1 || first.MediaAccountID != 10 || first.CompositeOutputID != 1 ||
		second.CategoryID != 2 || second.MediaAccountID != 9 || second.CompositeOutputID != 2 ||
		out.Shortfalls[0].Shortfall != 599 || out.Shortfalls[1].Shortfall != 0 {
		t.Fatalf("priority/shared limit/occupied handling = %+v", out)
	}
	dependency := errors.New("occupancy provider failed")
	svc.Repo = batchOccupancyFixture{err: dependency}
	if _, err = svc.AllocateBatch(context.Background(), a, in); !errors.Is(err, dependency) {
		t.Fatalf("dependency failure treated as empty: %v", err)
	}
	svc.Repo = batchOccupancyFixture{}
	in.Accounts[1].MediaAccountID = 11
	if _, err = svc.AllocateBatch(context.Background(), a, in); !errors.Is(err, ErrNotFound) {
		t.Fatalf("senior actor can select foreign account: %v", err)
	}
	a.Role = identity.RoleAdmin
	in.Accounts[1].MediaAccountID = 10
	if _, err = svc.AllocateBatch(context.Background(), a, in); !errors.Is(err, ErrNotFound) {
		t.Fatalf("administrator can prepare batch: %v", err)
	}
}

func TestBatchChangesOnlyInvalidateAffectedConfirmation(t *testing.T) {
	prior := []model.BatchCandidate{
		{RowKey: "unaffected", InputRevision: "1", Content: model.Content{Description: "title", Confirmed: true}},
		{RowKey: "changed", InputRevision: "1", FieldRevisions: map[string]string{"confirmed": "1"}, Content: model.Content{Description: "title", Confirmed: true}},
		{RowKey: "confirmed-again", InputRevision: "1", FieldRevisions: map[string]string{"confirmed": "1"}, Content: model.Content{Description: "title", Confirmed: true}},
	}
	current := append([]model.BatchCandidate(nil), prior...)
	current[0].Order = 5
	current[1].Content.Description = "changed title"
	current[1].FieldRevisions = map[string]string{"confirmed": "1", "description": "2"}
	current[2].Content.Description = "changed and reviewed"
	current[2].FieldRevisions = map[string]string{"confirmed": "2", "description": "2"}
	reconcileBatchConfirmations(prior, current)
	if !current[0].Content.Confirmed || current[1].Content.Confirmed || !current[2].Content.Confirmed {
		t.Fatalf("wrong confirmation projection: %+v", current)
	}
}

func TestBatchSelectedOutputMustBelongToTargetAndAuthorizedCategory(t *testing.T) {
	svc, a, _ := fixture()
	a.Role = identity.RoleSeniorOperator
	a.CategoryIDs = []identity.CategoryID{1, 2}
	svc.BatchCategory = func(id identity.CategoryID) (identity.Category, bool, error) {
		return identity.Category{ID: id, Enabled: true}, true, nil
	}
	original := svc.Output
	svc.Output = func(a identity.PublicUser, id int64) (production.ComposeOutputDetail, error) {
		got, err := original(a, id)
		got.Output.CategoryID = 2
		return got, err
	}
	in := BatchAllocationInput{Platform: "baidu", Targets: []model.BatchTarget{{CategoryID: 1, TargetCount: 3}}, BatchAllocation: model.BatchAllocation{Accounts: []model.BatchAccountLimit{}, OutputIDs: []string{"42"}}}
	if _, err := svc.AllocateBatch(context.Background(), a, in); !errors.Is(err, ErrInvalid) {
		t.Fatalf("output outside target accepted: %v", err)
	}
	settings := model.BatchExecutionSetting{ConcurrencyLimit: 1, ExceptionTaskLimit: 1, BetweenGroupPolicy: "auto_continue"}
	input := BatchInput{Platform: in.Platform, Targets: in.Targets, Config: model.BatchConfig{BatchAllocation: in.BatchAllocation, Candidates: []model.BatchCandidate{}}, ExecutionSetting: settings}
	if _, err := svc.checkBatch(context.Background(), a, input, nil, true); !errors.Is(err, ErrInvalid) {
		t.Fatalf("config output outside target allowed: %v", err)
	}
	a.CategoryIDs = []identity.CategoryID{1}
	if _, err := svc.AllocateBatch(context.Background(), a, in); !errors.Is(err, ErrNotFound) {
		t.Fatalf("output outside actor scope accepted: %v", err)
	}
}

type savedBatchFixture struct {
	BatchRepository
	batch model.Batch
}

func (r savedBatchFixture) GetBatch(id uint64) (model.Batch, error) {
	if id != r.batch.ID {
		return model.Batch{}, ErrNotFound
	}
	return r.batch, nil
}

func TestValidateBatchUsesOnlyAuthorizedRevisionBoundRetainedCovers(t *testing.T) {
	svc, a, _ := fixture()
	svc.BatchCategory = func(id identity.CategoryID) (identity.Category, bool, error) {
		return identity.Category{ID: id, Enabled: true}, true, nil
	}
	svc.Repo = batchOccupancyFixture{}
	svc.File = func(context.Context, identity.PublicUser, int64, int64) (filemodel.File, error) {
		return filemodel.File{}, images.ErrInvalid
	}
	savedCalls := 0
	svc.SavedFile = func(_ context.Context, id int64) (filemodel.File, error) {
		savedCalls++
		dimensions := []byte(`{"width":160,"height":90}`)
		if id == 22 {
			dimensions = []byte(`{"width":90,"height":120}`)
		}
		return filemodel.File{ID: id, Type: "image", SizeBytes: 1, SHA256: strings.Repeat("a", 64), MetadataJSON: dimensions}, nil
	}
	row := model.BatchCandidate{BatchPair: model.BatchPair{CategoryID: 1, MediaAccountID: 12, CompositeOutputID: 42}, RowKey: "row", InputRevision: "1", FieldRevisions: map[string]string{}, Content: model.Content{
		CoverFileID: 21, PortraitCoverFileID: 22, Description: "已确认文案", Declaration: "无需声明", Confirmed: true,
	}}
	target := []model.BatchTarget{{CategoryID: 1, TargetCount: 1}}
	saved := model.Batch{ID: 1, Revision: 3, TeamID: int64(*a.TeamID), OperatorID: int64(a.ID), ExecutionStatus: model.BatchNotStarted, Platform: "baidu", Targets: target, Config: model.BatchConfig{Candidates: []model.BatchCandidate{row}}}
	svc.BatchRepo = savedBatchFixture{batch: saved}
	config := BatchInput{Platform: "baidu", Targets: target, Config: model.BatchConfig{Candidates: []model.BatchCandidate{row}}, ExecutionSetting: model.BatchExecutionSetting{ConcurrencyLimit: 1, ExceptionTaskLimit: 1, BetweenGroupPolicy: "auto_continue"}}
	ctx := context.Background()
	req := ValidateBatchInput{BatchInput: config, BatchID: 1, ExpectedRevision: 3}
	result, err := svc.ValidateBatch(ctx, a, req)
	if err != nil || !result.Ready || savedCalls != 2 {
		t.Fatalf("valid current retained covers: %+v calls=%d err=%v", result, savedCalls, err)
	}
	req.ExpectedRevision = 2
	if _, err = svc.ValidateBatch(ctx, a, req); !errors.Is(err, ErrConflict) || savedCalls != 2 {
		t.Fatalf("stale validation: %v calls=%d", err, savedCalls)
	}
	req.ExpectedRevision = 3
	foreign := a
	foreign.ID++
	foreign.Role = identity.RoleSeniorOperator
	if _, err = svc.ValidateBatch(ctx, foreign, req); !errors.Is(err, ErrNotFound) || savedCalls != 2 {
		t.Fatalf("foreign validation: %v calls=%d", err, savedCalls)
	}
	req.Config.Candidates[0].Content.CoverFileID = 999
	result, err = svc.ValidateBatch(ctx, a, req)
	if err != nil || result.Ready || savedCalls != 3 {
		t.Fatalf("caller-supplied ID treated as retained: %+v calls=%d err=%v", result, savedCalls, err)
	}
}

func TestBatchReadinessKeepsOtherRowsWhenSelectedOutputBecomesUnreadable(t *testing.T) {
	ctx := context.Background()
	svc, a, quick := fixture()
	svc.BatchCategory = func(id identity.CategoryID) (identity.Category, bool, error) {
		return identity.Category{ID: id, Enabled: true}, true, nil
	}
	svc.Repo = batchOccupancyFixture{}
	savedOutput := svc.Output
	providerFailure := errors.New("production read failed")
	staleReason := production.ErrComposeOutputForbidden
	svc.Output = func(actor identity.PublicUser, id int64) (production.ComposeOutputDetail, error) {
		if id == 43 && actor.Role == identity.RoleOperator {
			return production.ComposeOutputDetail{}, staleReason
		}
		found, err := savedOutput(actor, id)
		found.Output.ID = id
		return found, err
	}
	protected := model.BatchCandidate{
		BatchPair: model.BatchPair{CategoryID: 1, MediaAccountID: 12, CompositeOutputID: 43},
		RowKey:    "protected", InputRevision: "1", FieldRevisions: map[string]string{}, Order: 0, Content: quick.Content,
	}
	available := protected
	available.RowKey, available.CompositeOutputID, available.Order = "available", 42, 1
	available.Content.Description = ""
	in := BatchInput{
		Platform: "baidu", Targets: []model.BatchTarget{{CategoryID: 1, TargetCount: 2}},
		Config: model.BatchConfig{
			BatchAllocation: model.BatchAllocation{Accounts: []model.BatchAccountLimit{{MediaAccountID: 12, Limit: 2}}, OutputIDs: []string{"43", "42"}},
			Candidates:      []model.BatchCandidate{protected, available},
		},
		ExecutionSetting: model.BatchExecutionSetting{ConcurrencyLimit: 1, ExceptionTaskLimit: 1, BetweenGroupPolicy: "auto_continue"},
	}
	v := model.Batch{
		ID: 4, Revision: 3, Name: "prepared", TeamID: int64(*a.TeamID), OperatorID: int64(a.ID), Platform: in.Platform, Targets: in.Targets,
		Config: in.Config, ExecutionSetting: in.ExecutionSetting, ExecutionStatus: model.BatchNotStarted,
	}
	checkIssues := func(t *testing.T, ready model.BatchReadiness, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("expected row issues, got request error: %v", err)
		}
		gotUnavailable, gotOther := false, false
		for _, issue := range ready.Issues {
			if issue.RowKey == "protected" && issue.Field == "composite_output_id" && issue.Code == "output_unavailable" {
				gotUnavailable = true
			}
			if issue.RowKey == "available" && issue.Code == "description_invalid" {
				gotOther = true
			}
		}
		if ready.Ready || !gotUnavailable || !gotOther {
			t.Fatalf("expected independent checks for both rows: %+v", ready)
		}
	}
	t.Run("validation", func(t *testing.T) {
		svc.BatchRepo = savedBatchFixture{batch: v}
		ready, err := svc.ValidateBatch(ctx, a, ValidateBatchInput{BatchInput: in, BatchID: v.ID, ExpectedRevision: v.Revision})
		checkIssues(t, ready, err)
	})
	t.Run("copy", func(t *testing.T) {
		ended := v
		ended.ExecutionStatus = model.BatchEnded
		svc.BatchRepo = savedBatchFixture{batch: ended}
		copied, err := svc.CopyBatch(ctx, a, ended.ID, ended.Revision)
		checkIssues(t, copied.Readiness, err)
	})
	t.Run("unpaired selection", func(t *testing.T) {
		newBatch := in
		newBatch.Config.Candidates = []model.BatchCandidate{available}
		ready, err := svc.ValidateBatch(ctx, a, ValidateBatchInput{BatchInput: newBatch})
		if err != nil {
			t.Fatal(err)
		}
		for _, issue := range ready.Issues {
			if issue.RowKey == "" && issue.Field == "output_ids" && issue.Code == "output_unavailable" {
				return
			}
		}
		t.Fatalf("missing generic error for unpaired stale selection: %+v", ready)
	})

	t.Run("strict writes and allocation", func(t *testing.T) {
		svc.BatchRepo = savedBatchFixture{batch: v}
		edit := EditBatchInput{BatchInput: in, Name: v.Name, ExpectedRevision: v.Revision}
		if _, err := svc.EditBatch(ctx, a, v.ID, edit); !errors.Is(err, staleReason) {
			t.Fatalf("edit accepted inaccessible selection: %v", err)
		}
		if _, err := svc.AllocateBatch(ctx, a, BatchAllocationInput{BatchAllocation: in.Config.BatchAllocation, Platform: in.Platform, Targets: in.Targets}); !errors.Is(err, staleReason) {
			t.Fatalf("allocation accepted inaccessible selection: %v", err)
		}
	})
	t.Run("unexpected dependency failures", func(t *testing.T) {
		staleReason = providerFailure
		svc.BatchRepo = savedBatchFixture{batch: v}
		if _, err := svc.ValidateBatch(ctx, a, ValidateBatchInput{BatchInput: in, BatchID: v.ID, ExpectedRevision: v.Revision}); !errors.Is(err, providerFailure) {
			t.Fatalf("masked dependency failure: %v", err)
		}
	})
}
