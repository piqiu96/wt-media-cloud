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
	if strategy.Status != "active" || strategy.Schedule == nil {
		return model.ComposeScheduleRun{}, fmt.Errorf("%w: strategy has no active plan", ErrStrategyInvalid)
	}
	if err := validateComposeSchedule(*strategy.Schedule); err != nil {
		return model.ComposeScheduleRun{}, err
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
