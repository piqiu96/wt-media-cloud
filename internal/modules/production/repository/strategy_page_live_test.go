package repository

import (
	"fmt"
	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	identity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
	"os"
	"testing"
	"time"
)

func TestLiveStrategyPageBeyond200AndScopedTotals(t *testing.T) {
	if os.Getenv("WT_MEDIA_TEST_LIVE_DB") != "1" {
		t.Skip("requires migrated local MySQL")
	}
	config.MustInitializeRuntimePaths()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err = database.Initialize(cfg.Databases); err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	tx := database.DB().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	var donorID int64
	if err = tx.Raw(`SELECT s.id FROM compose_strategies s JOIN compose_templates t ON t.id=s.compose_template_id AND t.version=s.compose_template_version WHERE s.operator_id IS NOT NULL AND s.status='enabled' AND t.status='active' LIMIT 1`).Row().Scan(&donorID); err != nil {
		t.Fatal(err)
	}
	donor, _, err := findComposeStrategy(tx, donorID)
	if err != nil {
		t.Fatal(err)
	}
	var foreignOwner, foreignTeam int64
	foreignCategory := donor.CategoryID
	if err = tx.Raw("SELECT id FROM users WHERE id<>? LIMIT 1", *donor.OperatorID).Row().Scan(&foreignOwner); err != nil {
		t.Fatal(err)
	}
	if err = tx.Raw("SELECT id FROM operation_teams WHERE id<>? LIMIT 1", donor.TeamID).Row().Scan(&foreignTeam); err != nil {
		t.Fatal(err)
	}
	if err = tx.Raw("SELECT id FROM categories WHERE id<>? LIMIT 1", donor.CategoryID).Row().Scan(&foreignCategory); err != nil {
		t.Fatal(err)
	}
	key := fmt.Sprintf("CHG118-page-%d", time.Now().UnixNano())
	now := time.Now().UTC()
	oldID := int64(0)
	insert := func(name, status string, team int64, category identity.CategoryID, owner any) int64 {
		t.Helper()
		err := tx.Exec(`INSERT INTO compose_strategies(team_id,category_id,operator_id,compose_template_id,compose_template_version,name,config_json,output_settings_json,execution_mode,priority_rules_json,single_limit,daily_limit,status,created_by,updated_by,created_at,updated_at) SELECT ?,?,?,compose_template_id,compose_template_version,?,config_json,output_settings_json,'manual',priority_rules_json,1,0,?,created_by,updated_by,?,? FROM compose_strategies WHERE id=?`, team, category, owner, name, status, now, now, donorID).Error
		if err != nil {
			t.Fatal(err)
		}
		var id int64
		if err = tx.Raw("SELECT id FROM compose_strategies WHERE name=?", name).Row().Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	for i := 0; i < 205; i++ {
		id := insert(fmt.Sprintf("%s-%03d", key, i), "enabled", donor.TeamID, donor.CategoryID, *donor.OperatorID)
		if i == 0 {
			oldID = id
		}
	}
	insert(key+"-disabled", "disabled", donor.TeamID, donor.CategoryID, *donor.OperatorID)
	insert(key+"-foreignowner", "enabled", donor.TeamID, donor.CategoryID, foreignOwner)
	insert(key+"-unowned", "enabled", donor.TeamID, donor.CategoryID, nil)
	insert(key+"-foreignteam", "enabled", foreignTeam, donor.CategoryID, *donor.OperatorID)
	insert(key+"-foreigncategory", "enabled", donor.TeamID, foreignCategory, *donor.OperatorID)
	filter := StrategyFilter{TeamID: donor.TeamID, OperatorID: *donor.OperatorID, CategoryIDs: []identity.CategoryID{donor.CategoryID}, ActiveOnly: true, Search: key}
	first, err := pageComposeStrategies(tx, filter, 1, 100)
	if err != nil || first.Total != 205 || len(first.Items) != 100 {
		t.Fatalf("first total=%d items=%d err=%v", first.Total, len(first.Items), err)
	}
	third, err := pageComposeStrategies(tx, filter, 3, 100)
	if err != nil || third.Total != 205 || len(third.Items) != 5 || third.Items[4].ID != oldID || third.Items[0].ID >= first.Items[99].ID {
		t.Fatalf("third=%+v old=%d err=%v", third, oldID, err)
	}
	filter.Search = fmt.Sprintf("%s-000", key)
	filter.CategoryID = donor.CategoryID
	filter.TemplateID = donor.TemplateID
	filter.Status = "enabled"
	old, err := pageComposeStrategies(tx, filter, 1, 20)
	if err != nil || old.Total != 1 || len(old.Items) != 1 || old.Items[0].ID != oldID {
		t.Fatalf("old=%+v err=%v", old, err)
	}
	filter.Search = fmt.Sprint(oldID)
	numeric, err := pageComposeStrategies(tx, filter, 1, 20)
	if err != nil || numeric.Total != 1 || len(numeric.Items) != 1 || numeric.Items[0].ID != oldID {
		t.Fatalf("numeric old=%+v err=%v", numeric, err)
	}
	filter.Search = key
	filter.Status = "disabled"
	empty, err := pageComposeStrategies(tx, filter, 1, 20)
	if err != nil || empty.Total != 0 {
		t.Fatalf("ordinary disabled=%+v err=%v", empty, err)
	}
	management, err := pageComposeStrategies(tx, StrategyFilter{TeamID: donor.TeamID, Search: key}, 1, 20)
	if err != nil || management.Total != 209 || len(management.Items) != 20 {
		t.Fatalf("management=%+v err=%v", management, err)
	}
	all, err := pageComposeStrategies(tx, StrategyFilter{Search: key}, 1, 20)
	if err != nil || all.Total != 210 {
		t.Fatalf("admin total=%d err=%v", all.Total, err)
	}
	foreign, err := pageComposeStrategies(tx, StrategyFilter{TeamID: foreignTeam, Search: key}, 1, 20)
	if err != nil || foreign.Total != 1 {
		t.Fatalf("foreign total=%d err=%v", foreign.Total, err)
	}
	beyond, err := pageComposeStrategies(tx, StrategyFilter{Search: key}, 20, 20)
	if err != nil || beyond.Total != 210 || len(beyond.Items) != 0 {
		t.Fatalf("beyond=%+v err=%v", beyond, err)
	}
}
