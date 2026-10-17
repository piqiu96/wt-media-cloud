package service

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
)

// PreviewComposeStrategyNextRun shares the scheduler's exact schedule algorithm.
func PreviewComposeStrategyNextRun(actor identity.PublicUser, input model.ComposeSchedule) (*time.Time, error) {
	if !canManageStrategies(actor) {
		return nil, ErrStrategyForbidden
	}
	return nextComposeRun(input, time.Now())
}

func nextRunForStrategy(item model.ComposeStrategy, now time.Time) (*time.Time, error) {
	if item.Status != model.ComposeStrategyEnabled || item.ExecutionMode != model.ComposeExecutionScheduled || item.Schedule == nil {
		return nil, nil
	}
	return nextComposeRun(*item.Schedule, now)
}

func validateComposeSchedule(s model.ComposeSchedule) error {
	if _, err := time.LoadLocation(s.Timezone); err != nil || strings.TrimSpace(s.Timezone) != s.Timezone || s.Timezone == "" {
		return fmt.Errorf("%w: timezone is invalid", ErrStrategyInvalid)
	}
	switch s.Mode {
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
