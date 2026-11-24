package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	accountdto "github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/dto"
	accounts "github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/service"
	outputmodel "github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	production "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
)

type BatchAccount struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Platform    string   `json:"platform"`
	CategoryIDs []string `json:"category_ids"`
	Eligible    bool     `json:"eligible"`
	Reason      string   `json:"reason"`
}
type BatchResource struct {
	AsOf                 time.Time `json:"as_of"`
	AccountCount         int64     `json:"account_count"`
	EligibleAccountCount int64     `json:"eligible_account_count"`
	OutputCount          int64     `json:"output_count"`
	AvailableOutputCount int64     `json:"available_output_count"`
	UnknownOutputCount   int64     `json:"unknown_output_count"`
	Availability         string    `json:"availability"`
	Gaps                 []string  `json:"gaps"`
}
type BatchAllocationInput struct {
	model.BatchAllocation
	Platform string              `json:"platform"`
	Targets  []model.BatchTarget `json:"targets"`
}
type BatchAllocationResult struct {
	Config     model.BatchConfig      `json:"config"`
	Shortfalls []model.BatchShortfall `json:"shortfalls"`
}

// Shared account readiness rule for single tasks and batch preparation.
func accountQualification(a identity.PublicUser, account accounts.Account, platform string, category int64) (string, error) {
	if account.UserID != a.ID {
		return "", ErrNotFound
	}
	if string(account.Platform) != platform {
		return "账号所属平台不匹配", nil
	}
	if !account.Enabled {
		return "账号已停用", nil
	}
	if account.IdentificationStatus != accounts.IdentificationIdentified || account.PlatformAccountID == "" {
		return "账号尚未完成识别", nil
	}
	if account.PlatformLoginStatus != accounts.PlatformLoginNormal {
		return "账号登录状态异常或未检查", nil
	}
	if account.BrowserProfileID == "" {
		return "账号尚未绑定浏览器环境", nil
	}
	if account.TeamID == nil {
		return "账号尚未关联团队", nil
	}
	if category > 0 {
		if !slices.Contains(account.CategoryIDs, identity.CategoryID(category)) || !a.CanAccess(account.UserID, account.TeamID, identity.CategoryID(category)) {
			return "", ErrNotFound
		}
	}
	return "", nil
}

func (s *Service) checkBatchCategory(a identity.PublicUser, id int64) error {
	if id <= 0 {
		return ErrInvalid
	}
	if !a.CanAccess(a.ID, a.TeamID, identity.CategoryID(id)) {
		return ErrNotFound
	}
	if s.BatchCategory == nil {
		return errors.New("category provider unavailable")
	}
	category, found, err := s.BatchCategory(identity.CategoryID(id))
	if err != nil {
		return err
	}
	if !found || !category.Enabled {
		return ErrNotFound
	}
	return nil
}

func (s *Service) AccountCandidates(a identity.PublicUser, platform string, category int64, search string, page, size int) (BatchPage[BatchAccount], error) {
	var empty BatchPage[BatchAccount]
	if !canPrepareBatch(a) {
		return empty, ErrNotFound
	}
	if platform == "" || !validBatchPagination(page, size) {
		return empty, ErrInvalid
	}
	if err := validateFilter(a, Filter{Page: page, PageSize: size, Search: search, Platform: platform, CategoryID: category}); err != nil {
		return empty, err
	}
	q := accounts.AccountFilter{Platform: accounts.Platform(platform), UserID: a.ID, Search: search}
	if category > 0 {
		if err := s.checkBatchCategory(a, category); err != nil {
			return empty, err
		}
		q.CategoryIDs = []identity.CategoryID{identity.CategoryID(category)}
	}
	if s.BatchAccounts == nil {
		return empty, errors.New("account list provider unavailable")
	}
	result, err := s.BatchAccounts(a, q, page, size)
	if err != nil {
		return empty, err
	}
	items := make([]BatchAccount, 0, len(result.Items))
	for _, account := range result.Items {
		// The owned filter is applied by the account service before pagination.
		// A provider violation should never disclose a foreign account.
		if account.UserID != a.ID {
			return empty, errors.New("account list scope mismatch")
		}
		reason, err := accountQualification(a, account, platform, category)
		if err != nil {
			reason = "账号不在当前发布类目范围内"
		}
		if account.TeamID != nil && *account.TeamID != *a.TeamID {
			reason = "账号不在当前团队范围内"
		}
		if platform != "baidu" {
			reason = "当前平台尚未开放批次准备"
		}
		visibleCategories := make([]string, 0, len(account.CategoryIDs))
		for _, id := range account.CategoryIDs {
			if a.CanAccess(account.UserID, account.TeamID, id) {
				visibleCategories = append(visibleCategories, strconv.FormatInt(int64(id), 10))
			}
		}
		name := account.PlatformName
		if name == "" {
			name = account.PlatformAccountID
		}
		if name == "" {
			name = "账号 " + account.ID
		}
		items = append(items, BatchAccount{ID: account.ID, Name: name, Platform: string(account.Platform), CategoryIDs: visibleCategories, Eligible: reason == "", Reason: reason})
	}
	return BatchPage[BatchAccount]{Items: items, Total: int64(result.Total), Page: page, PageSize: size}, nil
}

func (s *Service) BatchResources(ctx context.Context, a identity.PublicUser, platform string, category int64) (BatchResource, error) {
	var empty BatchResource
	if category <= 0 {
		return empty, ErrInvalid
	}
	result := BatchResource{AsOf: time.Now().UTC(), Availability: "empty", Gaps: []string{}}
	for page := 1; ; page++ {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		accounts, err := s.AccountCandidates(a, platform, category, "", page, 100)
		if err != nil {
			return empty, err
		}
		result.AccountCount = accounts.Total
		for _, account := range accounts.Items {
			if account.Eligible {
				result.EligibleAccountCount++
			}
		}
		if int64(page*100) >= accounts.Total || len(accounts.Items) == 0 {
			break
		}
	}
	for page := 1; ; page++ {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		outputs, err := s.OutputCandidates(ctx, a, production.ListComposeOutputsInput{Platform: platform, CategoryID: identity.CategoryID(category), Page: page, PageSize: 100})
		if err != nil {
			return empty, err
		}
		result.OutputCount = outputs.Total
		for _, output := range outputs.Items {
			if output.Occupied || !outputMatches(output.CompositeOutput, int64(*a.TeamID), category, platform) || output.PublishStatus == outputmodel.OutputPublishArchived {
				continue
			}
			if output.Video.Status == "unknown" {
				result.UnknownOutputCount++
			}
			if outputReady(output.CompositeOutput) {
				result.AvailableOutputCount++
			}
		}
		if int64(page*100) >= outputs.Total || len(outputs.Items) == 0 {
			break
		}
	}
	if result.EligibleAccountCount == 0 {
		result.Gaps = append(result.Gaps, "eligible_account_missing")
	}
	if result.AvailableOutputCount == 0 {
		result.Gaps = append(result.Gaps, "available_output_missing")
	}
	if result.UnknownOutputCount > 0 {
		result.Gaps = append(result.Gaps, "output_availability_unknown")
	}
	if result.EligibleAccountCount > 0 {
		if result.AvailableOutputCount > 0 {
			result.Availability = "available"
		} else if result.UnknownOutputCount > 0 {
			result.Availability = "unknown"
		}
	}
	return result, nil
}

type batchInventory struct {
	accounts  map[uint64]accounts.Account
	outputs   map[int64]outputmodel.CompositeOutput
	limits    map[uint64]int
	outputIDs []int64
}

func (v batchInventory) account(s *Service, a identity.PublicUser, id uint64) (accounts.Account, error) {
	if account, ok := v.accounts[id]; ok {
		return account, nil
	}
	if s.Account == nil {
		return accounts.Account{}, errors.New("account provider unavailable")
	}
	account, err := s.Account(a, strconv.FormatUint(id, 10))
	if err == nil && (account.UserID != a.ID || account.TeamID != nil && *account.TeamID != *a.TeamID) {
		return accounts.Account{}, ErrNotFound
	}
	if err == nil {
		v.accounts[id] = account
	}
	return account, err
}
func (v batchInventory) output(s *Service, a identity.PublicUser, id int64) (outputmodel.CompositeOutput, error) {
	if result, ok := v.outputs[id]; ok {
		return result, nil
	}
	if s.Output == nil {
		return outputmodel.CompositeOutput{}, errors.New("output provider unavailable")
	}
	detail, err := s.Output(a, id)
	if err == nil {
		v.outputs[id] = detail.Output
	}
	return detail.Output, err
}

func (s *Service) loadAllocation(ctx context.Context, a identity.PublicUser, platform string, targets map[int64]int64, in model.BatchAllocation, authorizeSelection bool) (batchInventory, error) {
	v := batchInventory{accounts: map[uint64]accounts.Account{}, outputs: map[int64]outputmodel.CompositeOutput{}, limits: map[uint64]int{}, outputIDs: make([]int64, 0, len(in.OutputIDs))}
	if len(in.Accounts) > s.batchLimits().MaxCandidates || len(in.OutputIDs) > s.batchLimits().MaxCandidates || len(in.ManualPairs) > s.batchLimits().MaxCandidates {
		return v, ErrInvalid
	}
	for _, account := range in.Accounts {
		if err := ctx.Err(); err != nil {
			return v, err
		}
		if account.MediaAccountID == 0 || account.Limit <= 0 || int64(account.Limit) > maxSafeTarget || v.limits[account.MediaAccountID] != 0 {
			return v, ErrInvalid
		}
		if authorizeSelection {
			record, err := v.account(s, a, account.MediaAccountID)
			if err != nil {
				return v, err
			}
			if string(record.Platform) != platform {
				return v, ErrInvalid
			}
		}
		v.limits[account.MediaAccountID] = account.Limit
	}
	chosen := make(map[int64]bool, len(in.OutputIDs))
	for _, raw := range in.OutputIDs {
		if err := ctx.Err(); err != nil {
			return v, err
		}
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 || strconv.FormatInt(id, 10) != raw || chosen[id] {
			return v, ErrInvalid
		}
		if authorizeSelection {
			actual, err := v.output(s, a, id)
			if err != nil {
				return v, err
			}
			if !outputMatches(actual, int64(*a.TeamID), int64(actual.CategoryID), platform) {
				return v, ErrInvalid
			}
		}
		chosen[id] = true
		v.outputIDs = append(v.outputIDs, id)
	}
	manual := make(map[int64]bool, len(in.ManualPairs))
	manualUsage := map[uint64]int{}
	for _, pair := range in.ManualPairs {
		if err := ctx.Err(); err != nil {
			return v, err
		}
		if _, has := targets[pair.CategoryID]; !has || pair.MediaAccountID == 0 || pair.CompositeOutputID <= 0 || manual[pair.CompositeOutputID] ||
			len(chosen) > 0 && !chosen[pair.CompositeOutputID] {
			return v, ErrInvalid
		}
		manualUsage[pair.MediaAccountID]++
		if len(v.limits) > 0 && manualUsage[pair.MediaAccountID] > v.limits[pair.MediaAccountID] {
			return v, ErrInvalid
		}
		if authorizeSelection {
			account, err := v.account(s, a, pair.MediaAccountID)
			if err != nil {
				return v, err
			}
			if _, err = accountQualification(a, account, platform, pair.CategoryID); err != nil {
				return v, err
			}
			if string(account.Platform) != platform {
				return v, ErrInvalid
			}
			output, err := v.output(s, a, pair.CompositeOutputID)
			if err != nil {
				return v, err
			}
			if !outputMatches(output, int64(*a.TeamID), pair.CategoryID, platform) {
				return v, ErrInvalid
			}
		}
		manual[pair.CompositeOutputID] = true
	}
	return v, nil
}

func (s *Service) batchOccupied(platform string, ids []int64) (map[int64]bool, error) {
	occupied := map[int64]bool{}
	if len(ids) == 0 {
		return occupied, nil
	}
	reader, ok := s.Repo.(outputOccupancyReader)
	if !ok {
		return nil, errors.New("output occupancy reader unavailable")
	}
	ids, err := reader.OccupiedOutputIDs(platform, ids)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		occupied[id] = true
	}
	return occupied, nil
}

func (s *Service) AllocateBatch(ctx context.Context, a identity.PublicUser, in BatchAllocationInput) (BatchAllocationResult, error) {
	var empty BatchAllocationResult
	targets, err := s.batchTargets(a, in.Platform, in.Targets)
	if err != nil {
		return empty, err
	}
	if in.Accounts == nil || in.OutputIDs == nil {
		return empty, ErrInvalid
	}
	inventory, err := s.loadAllocation(ctx, a, in.Platform, targets, in.BatchAllocation, true)
	if err != nil {
		return empty, err
	}
	ids := append([]int64(nil), inventory.outputIDs...)
	for _, pair := range in.ManualPairs {
		ids = append(ids, pair.CompositeOutputID)
	}
	occupied, err := s.batchOccupied(in.Platform, ids)
	if err != nil {
		return empty, err
	}
	selected := map[int64]bool{}
	usage := map[uint64]int{}
	rows := map[int64][]model.BatchCandidate{}
	add := func(pair model.BatchPair) {
		selected[pair.CompositeOutputID] = true
		usage[pair.MediaAccountID]++
		rows[pair.CategoryID] = append(rows[pair.CategoryID], model.BatchCandidate{
			BatchPair: pair, RowKey: fmt.Sprintf("row-%d", pair.CompositeOutputID), InputRevision: "1", FieldRevisions: map[string]string{},
		})
	}
	for _, pair := range in.ManualPairs {
		if inventory.limits[pair.MediaAccountID] == 0 || !slices.Contains(inventory.outputIDs, pair.CompositeOutputID) {
			return empty, ErrInvalid
		}
		reason, err := accountQualification(a, inventory.accounts[pair.MediaAccountID], in.Platform, pair.CategoryID)
		if err != nil {
			return empty, err
		}
		if occupied[pair.CompositeOutputID] {
			return empty, ErrOccupied
		}
		if reason != "" || !outputReady(inventory.outputs[pair.CompositeOutputID]) || int64(len(rows[pair.CategoryID])) >= targets[pair.CategoryID] {
			return empty, ErrInvalid
		}
		add(pair)
	}
	available := make(map[int64][]int64, len(in.Targets))
	for _, outputID := range inventory.outputIDs {
		output := inventory.outputs[outputID]
		categoryID := int64(output.CategoryID)
		if targets[categoryID] > 0 && !selected[outputID] && !occupied[outputID] && outputReady(output) {
			available[categoryID] = append(available[categoryID], outputID)
		}
	}
	for _, target := range in.Targets {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		categoryID := target.CategoryID
		pending := available[categoryID]
		for _, entry := range in.Accounts {
			remaining := target.TargetCount - int64(len(rows[categoryID]))
			if remaining <= 0 || len(pending) == 0 {
				break
			}
			reason, err := accountQualification(a, inventory.accounts[entry.MediaAccountID], in.Platform, categoryID)
			if err != nil || reason != "" {
				continue
			}
			count := min(int64(entry.Limit-usage[entry.MediaAccountID]), remaining, int64(len(pending)))
			for _, outputID := range pending[:count] {
				add(model.BatchPair{CategoryID: categoryID, MediaAccountID: entry.MediaAccountID, CompositeOutputID: outputID})
			}
			pending = pending[count:]
		}
	}
	config := model.BatchConfig{BatchAllocation: in.BatchAllocation, Candidates: []model.BatchCandidate{}}
	for _, target := range in.Targets {
		for _, row := range rows[target.CategoryID] {
			row.Order = len(config.Candidates)
			config.Candidates = append(config.Candidates, row)
		}
	}
	return BatchAllocationResult{Config: config, Shortfalls: shortfalls(in.Targets, config.Candidates)}, nil
}

type batchAccountPage func(identity.PublicUser, accounts.AccountFilter, int, int) (accountdto.AccountPage, error)
