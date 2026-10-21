package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
)

type ComposeScheduleRepository interface {
	DueStrategyIDs(time.Time) ([]int64, error)
	Run(repository.ScheduleRunOptions) (model.ComposeScheduleRun, bool, error)
	ListRuns(int64, int) ([]model.ComposeScheduleRun, error)
	ListRunHistory(repository.ScheduleRunHistoryOptions) (model.ComposeScheduleRunHistoryPage, error)
	BatchTasks(int64, time.Time) ([]model.ComposeTask, model.ComposeBatchCounts, error)
	FindRun(int64, time.Time) (model.ComposeScheduleRun, bool, error)
}

type ComposeSchedulerService struct {
	store      ComposeScheduleRepository
	strategies interface {
		Find(int64) (model.ComposeStrategy, bool, error)
	}
	now func() time.Time
}

func NewComposeSchedulerService(store ComposeScheduleRepository, strategies interface {
	Find(int64) (model.ComposeStrategy, bool, error)
}) *ComposeSchedulerService {
	return &ComposeSchedulerService{store: store, strategies: strategies, now: time.Now}
}

func RunDueComposeSchedules(now time.Time) (int, error) {
	return NewComposeSchedulerService(repository.ComposeScheduleStore{}, repository.StrategyStore{}).RunDue(now)
}

func RunComposeStrategyNow(actor identity.PublicUser, strategyID int64, requestKey string) (model.ComposeScheduleRun, error) {
	return NewComposeSchedulerService(repository.ComposeScheduleStore{}, repository.StrategyStore{}).RunNow(actor, strategyID, requestKey)
}

func ListComposeStrategyRuns(actor identity.PublicUser, strategyID int64) ([]model.ComposeScheduleRun, error) {
	return NewComposeSchedulerService(repository.ComposeScheduleStore{}, repository.StrategyStore{}).ListRuns(actor, strategyID)
}

func ListComposeStrategyRunHistory(actor identity.PublicUser, strategyID int64, options repository.ScheduleRunHistoryOptions) (model.ComposeScheduleRunHistoryPage, error) {
	return NewComposeSchedulerService(repository.ComposeScheduleStore{}, repository.StrategyStore{}).RunHistory(actor, strategyID, options)
}

func (s *ComposeSchedulerService) RunHistory(actor identity.PublicUser, strategyID int64, options repository.ScheduleRunHistoryOptions) (model.ComposeScheduleRunHistoryPage, error) {
	if strategyID <= 0 || actor.Status != identity.UserStatusEnabled {
		return model.ComposeScheduleRunHistoryPage{}, ErrStrategyNotFound
	}
	strategy, found, err := s.strategies.Find(strategyID)
	if err != nil {
		return model.ComposeScheduleRunHistoryPage{}, err
	}
	if !found || !canReadStrategy(actor, strategy) {
		return model.ComposeScheduleRunHistoryPage{}, ErrStrategyNotFound
	}
	options.StrategyID = strategyID
	return s.store.ListRunHistory(options)
}

func (s *ComposeSchedulerService) ListRuns(actor identity.PublicUser, strategyID int64) ([]model.ComposeScheduleRun, error) {
	if strategyID <= 0 || actor.Status != identity.UserStatusEnabled {
		return nil, ErrStrategyNotFound
	}
	strategy, found, err := s.strategies.Find(strategyID)
	if err != nil {
		return nil, err
	}
	if !found || !canReadStrategy(actor, strategy) {
		return nil, ErrStrategyNotFound
	}
	return s.store.ListRuns(strategyID, 20)
}

// ComposeBatchView is one schedule run's result surface: the run's own facts
// (skips included), every task it created for the slot, and per-status
// counts read from the rows themselves.
type ComposeBatchView struct {
	Run    model.ComposeScheduleRun `json:"run"`
	Tasks  []model.ComposeTask      `json:"tasks"`
	Counts model.ComposeBatchCounts `json:"counts"`
}

func (s *ComposeSchedulerService) Batch(actor identity.PublicUser, strategyID int64, slot time.Time) (ComposeBatchView, error) {
	if strategyID <= 0 || slot.IsZero() || actor.Status != identity.UserStatusEnabled {
		return ComposeBatchView{}, ErrStrategyNotFound
	}
	strategy, found, err := s.strategies.Find(strategyID)
	if err != nil {
		return ComposeBatchView{}, err
	}
	if !found || !canReadStrategy(actor, strategy) {
		return ComposeBatchView{}, ErrStrategyNotFound
	}
	tasks, counts, err := s.store.BatchTasks(strategyID, slot)
	if err != nil {
		return ComposeBatchView{}, err
	}
	run, _, err := s.store.FindRun(strategyID, slot)
	if err != nil {
		return ComposeBatchView{}, err
	}
	return ComposeBatchView{Run: run, Tasks: tasks, Counts: counts}, nil
}

func GetComposeStrategyBatch(actor identity.PublicUser, strategyID int64, slot time.Time) (ComposeBatchView, error) {
	return NewComposeSchedulerService(repository.ComposeScheduleStore{}, repository.StrategyStore{}).Batch(actor, strategyID, slot)
}

func (s *ComposeSchedulerService) RunDue(now time.Time) (int, error) {
	ids, err := s.store.DueStrategyIDs(now)
	if err != nil {
		return 0, err
	}
	processed := 0
	var failures []error
	for _, id := range ids {
		_, done, err := s.store.Run(repository.ScheduleRunOptions{StrategyID: id, Now: now, NextRun: nextRunForStrategy})
		if err != nil {
			failures = append(failures, fmt.Errorf("strategy %d: %w", id, err))
			continue
		}
		if done {
			processed++
		}
	}
	return processed, errors.Join(failures...)
}

func (s *ComposeSchedulerService) RunNow(actor identity.PublicUser, strategyID int64, requestKey string) (model.ComposeScheduleRun, error) {
	if !canManageStrategies(actor) {
		return model.ComposeScheduleRun{}, ErrStrategyForbidden
	}
	requestKey = strings.TrimSpace(requestKey)
	if len(requestKey) < 8 || len(requestKey) > 64 {
		return model.ComposeScheduleRun{}, fmt.Errorf("%w: request_key must be 8-64 characters", ErrStrategyInvalid)
	}
	strategy, found, err := s.strategies.Find(strategyID)
	if err != nil {
		return model.ComposeScheduleRun{}, err
	}
	if !found || !inGameScope(actor, strategy.TeamID, strategy.GameID) {
		return model.ComposeScheduleRun{}, ErrStrategyNotFound
	}
	if strategy.Status != model.ComposeStrategyEnabled {
		return model.ComposeScheduleRun{}, fmt.Errorf("%w: strategy is disabled", ErrStrategyInvalid)
	}
	if err := model.ValidateComposePriorityRules(strategy.PriorityRules); err != nil {
		return model.ComposeScheduleRun{}, fmt.Errorf("%w: %v", ErrStrategyInvalid, err)
	}
	if strategy.ExecutionMode == model.ComposeExecutionScheduled {
		if strategy.Schedule == nil {
			return model.ComposeScheduleRun{}, fmt.Errorf("%w: scheduled strategy has no plan", ErrStrategyInvalid)
		}
		if err := validateComposeSchedule(*strategy.Schedule); err != nil {
			return model.ComposeScheduleRun{}, err
		}
	}
	result, processed, err := s.store.Run(repository.ScheduleRunOptions{StrategyID: strategyID, Now: s.now(), ManualKey: requestKey, NextRun: nextRunForStrategy})
	if err != nil {
		return model.ComposeScheduleRun{}, err
	}
	if !processed {
		return model.ComposeScheduleRun{}, fmt.Errorf("%w: strategy is not available", ErrStrategyInvalid)
	}
	return result, nil
}
