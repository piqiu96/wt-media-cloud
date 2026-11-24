package repository_test

import (
	"fmt"
	"testing"
)

func TestBatchExecutionMigrationRetainsLegacyHoldersAndClosesCheckCutover(t *testing.T) {
	db := isolatedBeforeBatchExecution(t)
	for index, state := range []string{"running", "awaiting_human", "uncertain"} {
		profileID, outputID := index+37, index+42
		if err := db.Exec("INSERT INTO browser_profiles(id) VALUES(?)", profileID).Error; err != nil {
			t.Fatal(err)
		}
		taskKey := fmt.Sprintf("legacy-%d", index)
		if err := db.Exec(`INSERT INTO publication_task(id,team_id,operator_id,platform,category_id,media_account_id,composite_output_id,
   platform_account_uid,cover_file_id,resource_snapshot_json,content_json,status,created_by,request_key,request_hash,created_at,updated_at)
   VALUES(?,7,9,'baidu',1,12,?,?,1,JSON_OBJECT('browser_profile_id',CAST(? AS CHAR)),JSON_OBJECT(),'pending',9,?,REPEAT('a',64),NOW(6),NOW(6))`, index+1, outputID, taskKey, profileID, taskKey).Error; err != nil {
			t.Fatal(err)
		}
		attemptState := state
		if state == "uncertain" {
			attemptState = "interrupted"
		}
		if err := db.Exec(`INSERT INTO publication_attempt(id,publication_id,attempt_no,executor_user_id,browser_profile_id,
   claim_request_key,input_snapshot_json,status,stage,started_at,heartbeat_at,steps_json,evidence_json,admission_slot)
   VALUES(?,?,1,9,?,?,JSON_OBJECT('bit_main_user_id','actual-main','bit_profile_id',?),?, 'claimed',NOW(6),NOW(6),JSON_ARRAY(),JSON_OBJECT(),?)`,
			index+1, index+1, profileID, taskKey, fmt.Sprint("actual-profile-", profileID), attemptState, index+1).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Exec("UPDATE publication_task SET status=?,current_attempt_id=?,check_generation=1 WHERE id=?", state, index+1, index+1).Error; err != nil {
			t.Fatal(err)
		}
	}
	applyFileMigration(t, db, "20261125_117_publication_batch_execution.sql")
	var holds, automatic int64
	if err := db.Table("publication_profile_use").Where("kind='publish' AND released_at IS NULL").Count(&holds).Error; err != nil || holds != 3 {
		t.Fatalf("legacy profile holders %d: %v", holds, err)
	}
	if err := db.Table("publication_attempt").Where("automatic_slot=admission_slot AND reliably_parked_at IS NULL AND admission_v2=0").Count(&automatic).Error; err != nil || automatic != 3 {
		t.Fatalf("legacy automatic holders %d: %v", automatic, err)
	}
	var gate struct {
		LegacyCheckCount int64
		VerifiedAt       *string
	}
	if err := db.Table("publication_admission_cutover").Take(&gate).Error; err != nil || gate.LegacyCheckCount != 3 || gate.VerifiedAt != nil {
		t.Fatalf("legacy check cutover should stay closed: %+v %v", gate, err)
	}
}
