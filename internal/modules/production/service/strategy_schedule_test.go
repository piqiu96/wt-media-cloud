package service

import (
	"errors"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
)

func TestNextComposeRunIntervalAndDaily(t *testing.T) {
	now := time.Date(2026, 10, 14, 15, 30, 0, 0, time.UTC)
	cases := []struct {
		name string
		s    model.ComposeSchedule
		want time.Time
	}{
		{"interval", model.ComposeSchedule{Mode: "interval", IntervalMinutes: 30, Timezone: "Asia/Shanghai"}, now.Add(30 * time.Minute)},
		{"daily next day", model.ComposeSchedule{Mode: "daily", DailyTime: "22:00", Timezone: "Asia/Shanghai"}, time.Date(2026, 10, 15, 14, 0, 0, 0, time.UTC)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := nextComposeRun(tc.s, now)
			if err != nil || got == nil || !got.Equal(tc.want) {
				t.Fatalf("next=%v err=%v want=%v", got, err, tc.want)
			}
		})
	}
}

func TestPreviewComposeStrategyNextRunUsesScheduleValidationAndRole(t *testing.T) {
	_, _, actor := strategyFixture()
	schedule := model.ComposeSchedule{Mode: "interval", IntervalMinutes: 15, Timezone: "Asia/Shanghai"}
	actor.Role = "operator"
	if _, err := PreviewComposeStrategyNextRun(actor, schedule); !errors.Is(err, ErrStrategyForbidden) {
		t.Fatalf("operator preview error=%v", err)
	}
	actor.Role = "senior_operator"
	schedule.IntervalMinutes = 0
	if _, err := PreviewComposeStrategyNextRun(actor, schedule); !errors.Is(err, ErrStrategyInvalid) {
		t.Fatalf("invalid preview error=%v", err)
	}
	schedule.IntervalMinutes = 15
	if next, err := PreviewComposeStrategyNextRun(actor, schedule); err != nil || next == nil || !next.After(time.Now()) {
		t.Fatalf("next=%v err=%v", next, err)
	}
}

func TestComposeScheduleRejectsInvalidSettings(t *testing.T) {
	base := model.ComposeSchedule{Mode: "daily", DailyTime: "09:30", Timezone: "Asia/Shanghai"}
	cases := []struct {
		name string
		edit func(*model.ComposeSchedule)
	}{
		{"timezone", func(s *model.ComposeSchedule) { s.Timezone = "Mars/Olympus" }},
		{"daily time", func(s *model.ComposeSchedule) { s.DailyTime = "25:00" }},
		{"interval", func(s *model.ComposeSchedule) { s.Mode = "interval"; s.DailyTime = ""; s.IntervalMinutes = 0 }},
		{"manual enabled", func(s *model.ComposeSchedule) { s.Mode = "manual" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := base
			tc.edit(&s)
			if err := validateComposeSchedule(s); !errors.Is(err, ErrStrategyInvalid) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestComposeScheduleRespectsStrategyStatusAndCopy(t *testing.T) {
	svc, store, actor := strategyFixture()
	now := time.Date(2026, 10, 14, 8, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	store.items = []model.ComposeStrategy{{ID: 4, TeamID: 7, GameID: "delta", TemplateID: 3, TemplateVersion: 1, Name: "原策略", Config: []byte(`{"start_seconds":5,"end_seconds":12}`), ExecutionMode: model.ComposeExecutionManual, PriorityRules: strategyRulesFixture(), SingleLimit: 1, DailyLimit: 2, Status: "enabled", CreatedBy: 9}}
	schedule := model.ComposeSchedule{Mode: "interval", IntervalMinutes: 15, Timezone: "Asia/Shanghai"}
	updated, err := svc.Update(actor, 4, UpdateStrategyInput{Name: "原策略", Config: []byte(`{"start_seconds":5,"end_seconds":12}`), ExecutionMode: model.ComposeExecutionScheduled, Schedule: &schedule, PriorityRules: strategyRulesFixture(), SingleLimit: 1, DailyLimit: 2})
	if err != nil || updated.NextRunAt == nil || !updated.NextRunAt.After(now) {
		t.Fatalf("updated=%+v err=%v", updated, err)
	}
	inactive, err := svc.SetStatus(actor, 4, "disabled")
	if err != nil || inactive.NextRunAt != nil || inactive.Schedule == nil || inactive.ExecutionMode != model.ComposeExecutionScheduled {
		t.Fatalf("inactive=%+v err=%v", inactive, err)
	}
	active, err := svc.SetStatus(actor, 4, "enabled")
	if err != nil || active.NextRunAt == nil || !active.NextRunAt.After(now) {
		t.Fatalf("active=%+v err=%v", active, err)
	}
	copied, err := svc.Copy(actor, 4, "副本")
	if err != nil || copied.Schedule != nil || copied.NextRunAt != nil {
		t.Fatalf("copied=%+v err=%v", copied, err)
	}
	actor.Role = "operator"
	if _, err := svc.Update(actor, 4, UpdateStrategyInput{Name: "原策略", Config: []byte(`{"start_seconds":5,"end_seconds":12}`), ExecutionMode: model.ComposeExecutionScheduled, Schedule: &schedule, PriorityRules: strategyRulesFixture(), SingleLimit: 1, DailyLimit: 2}); !errors.Is(err, ErrStrategyForbidden) {
		t.Fatalf("operator error=%v", err)
	}
}
