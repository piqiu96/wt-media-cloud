package model

import "time"

type Counts struct {
	View     *uint64 `json:"view"`
	Like     *uint64 `json:"like"`
	Favorite *uint64 `json:"favorite"`
	Coin     *uint64 `json:"coin"`
	Reply    *uint64 `json:"reply"`
	Danmaku  *uint64 `json:"danmaku"`
}

func (v Counts) Valid() bool {
	for _, n := range []*uint64{v.View, v.Like, v.Favorite, v.Coin, v.Reply, v.Danmaku} {
		if n == nil || *n > 9007199254740991 {
			return false
		}
	}
	return true
}

type Metrics struct {
	WorkID        uint64 `json:"-" gorm:"primaryKey"`
	Counts        `gorm:"embedded"`
	CollectedAt   *time.Time `json:"collected_at"`
	LastAttemptAt *time.Time `json:"last_attempt_at"`
	Status        string     `json:"status"`
	LastError     string     `json:"last_error"`
	Source        string     `json:"source"`
	NextAttemptAt time.Time  `json:"-"`
	Failures      uint       `json:"-"`
	LeaseToken    string     `json:"-"`
	LeaseUntil    *time.Time `json:"-"`
}

func (Metrics) TableName() string { return "work_metric" }

type MetricSnapshot struct {
	ID          uint64
	WorkID      uint64
	Counts      `gorm:"embedded"`
	CollectedAt time.Time
}

func (MetricSnapshot) TableName() string { return "work_metric_snapshot" }

type MetricsClaim struct {
	WorkID             uint64 `json:"work_id,string"`
	PlatformWorkID     string `json:"platform_work_id"`
	PlatformAccountUID string `json:"platform_account_uid"`
	LeaseToken         string `json:"lease_token"`
}
type MetricsReport struct {
	LeaseToken         string  `json:"lease_token"`
	PlatformWorkID     string  `json:"platform_work_id"`
	PlatformAccountUID string  `json:"platform_account_uid"`
	Counts             *Counts `json:"counts"`
	Error              string  `json:"error"`
}
