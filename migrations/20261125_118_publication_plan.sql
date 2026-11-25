CREATE TABLE publication_plan (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
 team_id BIGINT UNSIGNED NOT NULL, owner_id BIGINT UNSIGNED NOT NULL, created_by BIGINT UNSIGNED NOT NULL,
 name VARCHAR(100) NOT NULL, date_start DATE NOT NULL, date_end DATE NOT NULL, note TEXT NOT NULL,
 goals_json JSON NOT NULL, status VARCHAR(24) NOT NULL DEFAULT 'not_started', pause_reason VARCHAR(200) NOT NULL DEFAULT '',
 current_batch_id BIGINT UNSIGNED NULL, started_at DATETIME(6) NULL, stopped_at DATETIME(6) NULL, ended_at DATETIME(6) NULL,
 end_reason VARCHAR(24) NOT NULL DEFAULT '', end_note TEXT NOT NULL,
 revision BIGINT UNSIGNED NOT NULL DEFAULT 1, request_key VARCHAR(128) COLLATE utf8mb4_bin NOT NULL,
 request_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 start_request_key VARCHAR(128) COLLATE utf8mb4_bin NULL, start_request_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
 start_receipt_json JSON NULL, handoff_receipts_json JSON NULL, resource_wait_json JSON NULL,
 created_at DATETIME(6) NOT NULL, updated_at DATETIME(6) NOT NULL,
 UNIQUE KEY uq_publication_plan_request(created_by,request_key),
 KEY idx_publication_plan_owner(owner_id,created_at,id), KEY idx_publication_plan_team(team_id,created_at,id), KEY idx_publication_plan_status(status,id),
 CONSTRAINT chk_publication_plan_status CHECK(status IN ('not_started','running','paused','stopping','ended')),
 CONSTRAINT chk_publication_plan_dates CHECK(date_end>=date_start)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
ALTER TABLE publication_batch
 ADD COLUMN plan_id BIGINT UNSIGNED NULL,
 ADD COLUMN plan_row_key VARCHAR(64) COLLATE utf8mb4_bin NULL,
 ADD COLUMN plan_group_order INT UNSIGNED NULL,
 ADD COLUMN plan_batch_order INT UNSIGNED NULL,
 ADD COLUMN plan_configured_at DATETIME(6) NULL,
 ADD COLUMN plan_frozen_at DATETIME(6) NULL,
 ADD COLUMN frozen_confirmation_json JSON NULL,
 ADD COLUMN handed_off_at DATETIME(6) NULL,
 ADD COLUMN handoff_basis_fingerprint CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
 ADD UNIQUE KEY uq_publication_plan_row(plan_id,plan_row_key),
 ADD UNIQUE KEY uq_publication_plan_order(plan_id,plan_group_order,plan_batch_order),
 ADD CONSTRAINT fk_publication_batch_plan FOREIGN KEY(plan_id) REFERENCES publication_plan(id);
ALTER TABLE publication_task ADD COLUMN latest_manual_verification_json JSON NULL;
