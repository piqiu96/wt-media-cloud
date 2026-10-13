package service

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
)

func SetComposeStrategySchedule(actor identity.PublicUser, id int64, input model.ComposeSchedule) (model.ComposeStrategy, error) {
	return wiredStrategyService().SetSchedule(actor, id, input)
}

func (s *StrategyService) SetSchedule(actor identity.PublicUser, id int64, input model.ComposeSchedule) (model.ComposeStrategy, error) {
	item, err := s.managed(actor, id)
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	if err := validateComposeSchedule(input); err != nil {
		return model.ComposeStrategy{}, err
	}
	if input.Enabled && item.Status == "active" {
		if err := s.validateConfig(item.TemplateID, item.TemplateVersion, item.Config, true); err != nil {
			return model.ComposeStrategy{}, err
		}
	}
	item.Schedule = &input
	item.NextRunAt, err = nextRunForStrategy(item, s.now())
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	item.UpdatedBy = int64(actor.ID)
	return s.store.UpdateAudited(item, "compose_strategy.schedule")
}

func nextRunForStrategy(item model.ComposeStrategy, now time.Time) (*time.Time, error) {
	if item.Status != "active" || item.Schedule == nil || !item.Schedule.Enabled {
		return nil, nil
	}
	return nextComposeRun(*item.Schedule, now)
}

func validateComposeSchedule(s model.ComposeSchedule) error {
	if s.MaterialScope != model.ComposeMaterialScopeCreatorUsages {
		return fmt.Errorf("%w: material_scope must be creator_usages", ErrStrategyInvalid)
	}
	if _, err := time.LoadLocation(s.Timezone); err != nil || strings.TrimSpace(s.Timezone) != s.Timezone || s.Timezone == "" {
		return fmt.Errorf("%w: timezone is invalid", ErrStrategyInvalid)
	}
	if s.PerRunLimit < 1 || s.PerRunLimit > 100 || s.DailyLimit < s.PerRunLimit || s.DailyLimit > 1000 {
		return fmt.Errorf("%w: limits must satisfy 1 <= per_run_limit <= 100 and per_run_limit <= daily_limit <= 1000", ErrStrategyInvalid)
	}
	switch s.Mode {
	case "manual":
		if s.Enabled || s.IntervalMinutes != 0 || s.DailyTime != "" {
			return fmt.Errorf("%w: manual plan cannot be enabled or timed", ErrStrategyInvalid)
		}
	case "interval":
		if s.IntervalMinutes < 1 || s.IntervalMinutes > 1440 || s.DailyTime != "" {
			return fmt.Errorf("%w: interval_minutes must be 1-1440", ErrStrategyInvalid)
		}
	case "daily":
		if s.IntervalMinutes != 0 {
			return fmt.Errorf("%w: interval_minutes must be empty for daily plan", ErrStrategyInvalid)
		}
		if _, _, err := parseComposeDailyTime(s.DailyTime); err != nil {
			return err
		}
	default:
		return fmt.Errorf("%w: mode is invalid", ErrStrategyInvalid)
	}
	return nil
}

func parseComposeDailyTime(value string) (int, int, error) {
	if len(value) != 5 || value[2] != ':' {
		return 0, 0, fmt.Errorf("%w: daily_time must be HH:MM", ErrStrategyInvalid)
	}
	hour, errHour := strconv.Atoi(value[:2])
	minute, errMinute := strconv.Atoi(value[3:])
	if errHour != nil || errMinute != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("%w: daily_time must be HH:MM", ErrStrategyInvalid)
	}
	return hour, minute, nil
}

func nextComposeRun(s model.ComposeSchedule, now time.Time) (*time.Time, error) {
	if err := validateComposeSchedule(s); err != nil {
		return nil, err
	}
	if !s.Enabled {
		return nil, nil
	}
	if s.Mode == "interval" {
		next := now.Add(time.Duration(s.IntervalMinutes) * time.Minute).UTC()
		return &next, nil
	}
	location, _ := time.LoadLocation(s.Timezone)
	hour, minute, _ := parseComposeDailyTime(s.DailyTime)
	local := now.In(location)
	for day := 0; day < 3; day++ {
		date := local.AddDate(0, 0, day)
		candidate := time.Date(date.Year(), date.Month(), date.Day(), hour, minute, 0, 0, location)
		if candidate.Hour() == hour && candidate.Minute() == minute && candidate.After(now) {
			next := candidate.UTC()
			return &next, nil
		}
	}
	return nil, fmt.Errorf("%w: daily time has no next occurrence", ErrStrategyInvalid)
}
