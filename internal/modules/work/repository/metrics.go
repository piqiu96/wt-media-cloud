package repository

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/work/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrMetricsLease = errors.New("work metrics lease lost")

func metricRow(tx *gorm.DB, id uint64, now time.Time) (model.Metrics, error) {
	v := model.Metrics{WorkID: id, Status: "queued", Source: "bilibili_public_api", NextAttemptAt: now}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&v).Error; err != nil {
		return v, err
	}
	err := tx.First(&v, "work_id=?", id).Error
	return v, err
}
func lockWork(tx *gorm.DB, id uint64) (model.Work, error) {
	var w model.Work
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&w, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = ErrNotFound
	}
	return w, err
}
func (s Store) RequestMetrics(id uint64, now time.Time) (out model.Metrics, err error) {
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if _, e := lockWork(tx, id); e != nil {
			return e
		}
		v, e := metricRow(tx, id, now)
		if e != nil {
			return e
		}
		if v.LeaseUntil != nil && v.LeaseUntil.After(now) || v.LastAttemptAt != nil && now.Sub(*v.LastAttemptAt) < 30*time.Second {
			out = v
			return nil
		}
		v.Status = "queued"
		v.NextAttemptAt = now
		if e = tx.Save(&v).Error; e != nil {
			return e
		}
		out = v
		return nil
	})
	return
}
func (s Store) ClaimMetrics(a identity.PublicUser, now time.Time) (out *model.MetricsClaim, err error) {
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		var w model.Work
		q := (Store{DB: tx}).query(Filter{Actor: a}).
			Joins("LEFT JOIN work_metric m ON m.work_id=work.id").
			Where("work.operator_id=? AND work.platform='bilibili'", a.ID).
			Where("(m.work_id IS NULL OR m.next_attempt_at<=?) AND (m.lease_until IS NULL OR m.lease_until<=?)", now, now).
			Select("work.*").Order("COALESCE(m.next_attempt_at,'1970-01-01'),work.id").
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
		if e := q.Take(&w).Error; errors.Is(e, gorm.ErrRecordNotFound) {
			return nil
		} else if e != nil {
			return e
		}
		v, e := metricRow(tx, w.ID, now)
		if e != nil {
			return e
		}
		key := make([]byte, 32)
		if _, e = rand.Read(key); e != nil {
			return e
		}
		until := now.Add(90 * time.Second)
		v.LeaseToken = hex.EncodeToString(key)
		v.LeaseUntil = &until
		v.LastAttemptAt = &now
		v.Status = "refreshing"
		if e = tx.Save(&v).Error; e != nil {
			return e
		}
		out = &model.MetricsClaim{WorkID: w.ID, PlatformWorkID: w.PlatformWorkID, PlatformAccountUID: w.PlatformAccountUID, LeaseToken: v.LeaseToken}
		return nil
	})
	return
}
func (s Store) ReportMetrics(user int64, id uint64, in model.MetricsReport, now time.Time) (out model.Metrics, err error) {
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		w, e := lockWork(tx, id)
		if e != nil {
			return e
		}
		if w.OperatorID != user || w.Platform != "bilibili" || w.PlatformWorkID != in.PlatformWorkID || w.PlatformAccountUID != in.PlatformAccountUID {
			return ErrMetricsLease
		}
		var v model.Metrics
		if e = tx.First(&v, "work_id=?", id).Error; errors.Is(e, gorm.ErrRecordNotFound) {
			return ErrMetricsLease
		} else if e != nil {
			return e
		}
		if v.LeaseToken != in.LeaseToken || v.LeaseUntil == nil || !v.LeaseUntil.After(now) {
			return ErrMetricsLease
		}
		v.LeaseToken = ""
		v.LeaseUntil = nil
		v.LastError = in.Error
		delay := time.Hour
		if in.Error == "" {
			v.Counts = *in.Counts
			v.CollectedAt = &now
			v.Status = "ready"
			v.Failures = 0
			snapshot := model.MetricSnapshot{WorkID: id, Counts: *in.Counts, CollectedAt: now}
			if e = tx.Create(&snapshot).Error; e != nil {
				return e
			}
		} else {
			v.Status = "failed"
			v.Failures++
			delay = 5 * time.Minute
			for n := uint(1); n < v.Failures && delay < time.Hour; n++ {
				delay *= 2
			}
			if delay > time.Hour {
				delay = time.Hour
			}
		}
		v.NextAttemptAt = now.Add(delay)
		if e = tx.Save(&v).Error; e != nil {
			return e
		}
		out = v
		return nil
	})
	return
}
