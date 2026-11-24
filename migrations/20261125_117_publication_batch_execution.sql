-- Stop legacy claim/check writers before applying. See the cutover instructions.
CREATE TABLE IF NOT EXISTS publication_profile_use (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
 bit_main_user_id VARBINARY(1020) NOT NULL,
 bit_profile_id VARBINARY(1020) NOT NULL,
 owner_id BIGINT UNSIGNED NOT NULL,
 publication_id BIGINT UNSIGNED NOT NULL,
 attempt_id BIGINT UNSIGNED NULL,
 kind VARCHAR(12) NOT NULL,
 check_generation BIGINT UNSIGNED NULL,
 check_revision BIGINT UNSIGNED NULL,
 check_browser_fallback_allowed BOOLEAN NOT NULL DEFAULT FALSE,
 window_slot SMALLINT UNSIGNED NOT NULL,
 acquired_at DATETIME(6) NOT NULL,
 released_at DATETIME(6) NULL,
 disposition VARCHAR(16) NULL,
 observation_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
 active_main_id VARBINARY(1020) GENERATED ALWAYS AS (IF(released_at IS NULL,bit_main_user_id,NULL)) STORED,
 active_profile_id VARBINARY(1020) GENERATED ALWAYS AS (IF(released_at IS NULL,bit_profile_id,NULL)) STORED,
 active_window_slot SMALLINT UNSIGNED GENERATED ALWAYS AS (IF(released_at IS NULL,window_slot,NULL)) STORED,
 UNIQUE KEY uq_publication_profile_use (active_main_id,active_profile_id),
 UNIQUE KEY uq_publication_window_slot (owner_id,active_window_slot),
 UNIQUE KEY uq_publication_check_use (publication_id,check_generation),
 KEY idx_publication_profile_attempt (attempt_id,kind,released_at),
 CONSTRAINT chk_publication_profile_use_subject CHECK (kind IN ('publish','check')),
 CONSTRAINT chk_publication_profile_use_identity CHECK (OCTET_LENGTH(bit_main_user_id)>0 AND OCTET_LENGTH(bit_profile_id)>0 AND window_slot>0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Serialize a pending original check request and its ReportSafe settlement
-- before any Profile is known. Unadmitted cancelled keys are tombstones.
CREATE TABLE IF NOT EXISTS publication_check_request (
 owner_id BIGINT UNSIGNED NOT NULL,
 request_key VARCHAR(64) COLLATE utf8mb4_bin NOT NULL,
 agent_session_id VARCHAR(128) COLLATE utf8mb4_bin NOT NULL,
 request_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
 reservation_id BIGINT UNSIGNED NULL,
 cancelled_at DATETIME(6) NULL,
 created_at DATETIME(6) NOT NULL,
 PRIMARY KEY (owner_id,request_key),
 UNIQUE KEY uq_publication_check_request_reservation(reservation_id),
 CONSTRAINT fk_publication_check_request_reservation FOREIGN KEY(reservation_id) REFERENCES publication_profile_use(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Missing identities and duplicate active actual Profiles deliberately stop
-- migration. This INSERT never drops or silently frees an old holder.
INSERT INTO publication_profile_use
 (bit_main_user_id,bit_profile_id,owner_id,publication_id,attempt_id,kind,window_slot,acquired_at)
SELECT
 IFNULL(NULLIF(JSON_UNQUOTE(JSON_EXTRACT(a.input_snapshot_json,'$.bit_main_user_id')),'null'),''),
 IFNULL(NULLIF(JSON_UNQUOTE(JSON_EXTRACT(a.input_snapshot_json,'$.bit_profile_id')),'null'),''),
 a.executor_user_id,a.publication_id,a.id,'publish',a.admission_slot,a.started_at
FROM publication_attempt a
WHERE (a.status IN ('running','awaiting_human') OR (a.status='interrupted' AND a.resolved_at IS NULL))
  AND NOT EXISTS (SELECT 1 FROM publication_profile_use p WHERE p.kind='publish' AND p.attempt_id=a.id);

CREATE TABLE IF NOT EXISTS publication_admission_cutover (
 id TINYINT UNSIGNED NOT NULL PRIMARY KEY,
 legacy_check_count BIGINT UNSIGNED NOT NULL,
 verified_at DATETIME(6) NULL,
 verification_reference VARCHAR(200) NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO publication_admission_cutover (id,legacy_check_count,verified_at,verification_reference)
SELECT 1,COUNT(*),IF(COUNT(*)=0,NOW(6),NULL),IF(COUNT(*)=0,'no legacy check generations',NULL)
FROM publication_task WHERE check_generation>0
ON DUPLICATE KEY UPDATE id=publication_admission_cutover.id;

ALTER TABLE publication_batch
 DROP CHECK chk_publication_batch_execution,
 ADD CONSTRAINT chk_publication_batch_execution CHECK(execution_status IN ('not_started','running','paused','stopping','ended','cancelled')),
 ADD COLUMN execution_group_no INT UNSIGNED NULL,
 ADD COLUMN group_ended_at DATETIME(6) NULL,
 ADD COLUMN next_group_at DATETIME(6) NULL,
 ADD COLUMN stopped_at DATETIME(6) NULL,
 ADD COLUMN ended_at DATETIME(6) NULL,
 ADD COLUMN stop_reason VARCHAR(200) NOT NULL DEFAULT '',
 ADD COLUMN finish_reason VARCHAR(200) NOT NULL DEFAULT '',
 ADD COLUMN pause_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
 ADD COLUMN pause_json JSON NULL,
 ADD COLUMN exception_ack_json JSON NULL,
 ADD COLUMN start_request_key VARCHAR(64) COLLATE utf8mb4_bin NULL,
 ADD COLUMN start_request_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
 ADD COLUMN start_receipt_json JSON NULL,
 ADD KEY idx_publication_batch_dispatch (operator_id,execution_status,id);

ALTER TABLE publication_task
 MODIFY COLUMN request_key VARCHAR(64) COLLATE utf8mb4_bin NULL,
 MODIFY COLUMN request_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
 ADD COLUMN batch_id BIGINT UNSIGNED NULL,
 ADD COLUMN task_order INT UNSIGNED NOT NULL DEFAULT 0,
 ADD COLUMN row_key VARCHAR(64) COLLATE utf8mb4_bin NULL,
 ADD COLUMN execution_group_no INT UNSIGNED NULL,
 ADD CONSTRAINT fk_publication_task_batch FOREIGN KEY(batch_id) REFERENCES publication_batch(id),
 ADD UNIQUE KEY uq_publication_batch_task_order(batch_id,task_order),
 ADD UNIQUE KEY uq_publication_batch_row(batch_id,row_key),
 ADD KEY idx_publication_batch_task_state(batch_id,status,id),
 ADD KEY idx_publication_batch_task_group(batch_id,execution_group_no,task_order);

ALTER TABLE publication_attempt
 DROP INDEX uq_attempt_user_slot,
 DROP COLUMN active_admission_slot,
 DROP CHECK chk_attempt_admission_slot,
 MODIFY COLUMN admission_slot SMALLINT UNSIGNED NOT NULL DEFAULT 1,
 ADD CONSTRAINT chk_attempt_admission_slot CHECK(admission_slot>0),
 ADD COLUMN active_admission_slot SMALLINT UNSIGNED GENERATED ALWAYS AS
   (CASE WHEN status IN ('running','awaiting_human') OR (status='interrupted' AND resolved_at IS NULL) THEN admission_slot ELSE NULL END) STORED,
 ADD UNIQUE KEY uq_attempt_user_slot (executor_user_id,active_admission_slot),
 ADD COLUMN execution_group_no INT UNSIGNED NULL,
 ADD COLUMN automatic_slot SMALLINT UNSIGNED NULL,
 ADD COLUMN reliably_parked_at DATETIME(6) NULL,
 ADD COLUMN admission_v2 BOOLEAN NOT NULL DEFAULT FALSE,
 ADD COLUMN active_automatic_slot SMALLINT UNSIGNED GENERATED ALWAYS AS
   (CASE WHEN status IN ('running','awaiting_human') OR (status='interrupted' AND resolved_at IS NULL) THEN automatic_slot ELSE NULL END) STORED,
 ADD UNIQUE KEY uq_publication_automatic_slot (executor_user_id,active_automatic_slot);

UPDATE publication_attempt
SET automatic_slot=admission_slot
WHERE status IN ('running','awaiting_human') OR (status='interrupted' AND resolved_at IS NULL);
