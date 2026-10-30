ALTER TABLE media_accounts
 ADD COLUMN last_check_status VARCHAR(32) NOT NULL DEFAULT 'never',
 ADD COLUMN last_check_message VARCHAR(500) NULL,
 ADD COLUMN last_check_identity JSON NULL,
 ADD COLUMN check_operation_id VARCHAR(64) NULL,
 ADD COLUMN check_profile_id BIGINT UNSIGNED NULL,
 ADD COLUMN check_started_at DATETIME(6) NULL;
