package repository_test

import (
	"context"
	"errors"
	"fmt"
	driver "github.com/go-sql-driver/mysql"
	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database/migration"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/work/model"
	repo "github.com/wt-media/wt-media-cloud/internal/modules/work/repository"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func metricStore(t *testing.T) repo.Store {
	t.Helper()
	if os.Getenv("WT_MEDIA_TEST_LIVE_DB") != "1" {
		t.Skip("isolated MySQL requires WT_MEDIA_TEST_LIVE_DB=1")
	}
	config.MustInitializeRuntimePaths()
	cfg, e := config.Load()
	if e != nil {
		t.Fatal(e)
	}
	c := cfg.Databases[0]
	d := driver.NewConfig()
	d.User = c.Username
	d.Passwd = c.Password
	d.Net = "tcp"
	d.Addr = fmt.Sprintf("%s:%d", c.Host, c.Port)
	d.ParseTime = true
	d.Loc = time.UTC
	root, e := gorm.Open(gormmysql.Open(d.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal(e)
	}
	name := fmt.Sprintf("wt_metrics_test_%d", time.Now().UnixNano())
	if e = root.Exec("CREATE DATABASE " + name).Error; e != nil {
		t.Fatal(e)
	}
	d.DBName = name
	db, e := gorm.Open(gormmysql.Open(d.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		sql, _ := db.DB()
		sql.Close()
		root.Exec("DROP DATABASE " + name)
		sql, _ = root.DB()
		sql.Close()
	})
	// The exact Work model columns, without unrelated publication foreign keys.
	workSchema, e := os.ReadFile(config.GetRuntimePaths().Migrations + "/20261121_112_public_work.sql")
	if e != nil {
		t.Fatal(e)
	}
	sql := string(workSchema)
	start := strings.Index(sql, "CREATE TABLE work (")
	if start < 0 {
		t.Fatal("work schema missing")
	}
	end := start + strings.Index(sql[start:], ";") + 1
	// Drop fixture-only FKs; indexes/column types remain the actual provider DDL.
	lines := []byte{}
	s := sql[start:end]
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, "CONSTRAINT fk_work_") {
			continue
		}
		lines = append(lines, []byte(line+"\n")...)
	}
	ddl := string(lines)
	ddl = strings.ReplaceAll(ddl, ",\n) ENGINE", "\n) ENGINE")
	if e = db.Exec(ddl).Error; e != nil {
		t.Fatal(e)
	}
	body, e := os.ReadFile(config.GetRuntimePaths().Migrations + "/20261127_119_work_metrics.sql")
	if e != nil {
		t.Fatal(e)
	}
	m := []migration.Migration{{Version: "119", Name: "metrics", SQL: string(body)}}
	if _, e = migration.Apply(context.Background(), db, m); e != nil {
		t.Fatal(e)
	}
	if applied, e := migration.Apply(context.Background(), db, m); e != nil || len(applied) != 0 {
		t.Fatal("migration replay", e)
	}
	return repo.Store{DB: db}
}
func TestLiveMetricsLifecycleAndLease(t *testing.T) {
	s := metricStore(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	team := identity.TeamID(7)
	a := identity.PublicUser{ID: 9, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, CategoryIDs: []identity.CategoryID{1}}
	w := model.Work{ID: 1, Platform: "bilibili", PlatformWorkID: "123", PlatformAccountUID: "456", OperatorID: 9, TeamID: 7, CategoryID: 1, PublishedAt: now, FirstObservedAt: now}
	for _, v := range []model.Work{w, func() model.Work { v := w; v.ID = 2; v.PlatformWorkID = "124"; v.OperatorID = 10; return v }(), func() model.Work { v := w; v.ID = 3; v.PlatformWorkID = "125"; v.CategoryID = 2; return v }()} {
		if e := s.DB.Create(&v).Error; e != nil {
			t.Fatal(e)
		}
	}
	var wg sync.WaitGroup
	claims := make(chan *model.MetricsClaim, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); c, e := s.ClaimMetrics(a, now); claims <- c; errs <- e }()
	}
	wg.Wait()
	close(claims)
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	var c *model.MetricsClaim
	n := 0
	for v := range claims {
		if v != nil {
			c = v
			n++
		}
	}
	if n != 1 || c.WorkID != 1 {
		t.Fatalf("concurrent/scope claims: %d %+v", n, c)
	}
	if v, e := s.RequestMetrics(1, now.Add(time.Second)); e != nil || v.Status != "refreshing" {
		t.Fatal("in-flight refresh not coalesced", v, e)
	}
	zero := uint64(0)
	seven := uint64(7)
	counts := model.Counts{View: &seven, Like: &zero, Favorite: &zero, Coin: &zero, Reply: &zero, Danmaku: &zero}
	in := model.MetricsReport{LeaseToken: c.LeaseToken, PlatformWorkID: "123", PlatformAccountUID: "456", Counts: &counts}
	if _, e := s.ReportMetrics(10, 1, in, now); !errors.Is(e, repo.ErrMetricsLease) {
		t.Fatal("foreign report", e)
	}
	good, e := s.ReportMetrics(9, 1, in, now.Add(time.Second))
	if e != nil || good.View == nil || *good.View != 7 {
		t.Fatal(good, e)
	}
	if _, e := s.ReportMetrics(9, 1, in, now.Add(2*time.Second)); !errors.Is(e, repo.ErrMetricsLease) {
		t.Fatal("replayed report", e)
	}
	loaded, e := s.Get(1)
	if e != nil || loaded.Metrics == nil || *loaded.Metrics.View != 7 {
		t.Fatal("detail preload", loaded, e)
	}
	rows, _, e := s.List(repo.Filter{Actor: a, Page: 1, PageSize: 20})
	if e != nil || len(rows) != 1 || rows[0].Metrics == nil {
		t.Fatal("list preload", rows, e)
	}
	if v, e := s.ClaimMetrics(a, now.Add(2*time.Second)); e != nil || v != nil {
		t.Fatal("not due claimed", v, e)
	}
	s.RequestMetrics(1, now.Add(3*time.Second))
	c, e = s.ClaimMetrics(a, now.Add(3*time.Second))
	if e != nil || c == nil {
		t.Fatal(c, e)
	}
	in.LeaseToken = c.LeaseToken
	in.Counts = nil
	in.Error = "rate_limited"
	bad, e := s.ReportMetrics(9, 1, in, now.Add(61*time.Second))
	if e != nil || *bad.View != 7 || !bad.CollectedAt.Equal(*good.CollectedAt) || bad.Status != "failed" {
		t.Fatal("failure lost success", bad, e)
	}
	if v, e := s.ClaimMetrics(a, now.Add(2*time.Minute)); e != nil || v != nil {
		t.Fatal("backoff ignored", v, e)
	}
	c, e = s.ClaimMetrics(a, now.Add(7*time.Minute))
	if e != nil || c == nil {
		t.Fatal(c, e)
	}
	old := c.LeaseToken
	c, e = s.ClaimMetrics(a, now.Add(9*time.Minute))
	if e != nil || c == nil || c.LeaseToken == old {
		t.Fatal("expired lease not recovered", c, e)
	}
	in.LeaseToken = old
	if _, e := s.ReportMetrics(9, 1, in, now.Add(9*time.Minute)); !errors.Is(e, repo.ErrMetricsLease) {
		t.Fatal("late report overwrote lease", e)
	}
	var snapshots int64
	s.DB.Model(&model.MetricSnapshot{}).Count(&snapshots)
	if snapshots != 1 {
		t.Fatal("failed/replayed reports made snapshots", snapshots)
	}
}
