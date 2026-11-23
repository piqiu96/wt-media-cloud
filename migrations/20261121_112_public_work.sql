-- Preserve real external receipts while separating public work identity.
ALTER TABLE publication_task
 RENAME COLUMN platform_work_id TO platform_submission_id,
 RENAME COLUMN work_url TO target_url,
 ADD COLUMN platform_work_id VARCHAR(255) COLLATE utf8mb4_bin NULL,
 ADD COLUMN work_url VARCHAR(2048) NULL,
 ADD COLUMN work_id BIGINT UNSIGNED NULL;
CREATE TABLE work (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
 platform VARCHAR(32) NOT NULL,
 platform_work_id VARCHAR(255) COLLATE utf8mb4_bin NOT NULL,
 work_url VARCHAR(2048) NOT NULL,
 platform_account_uid VARCHAR(255) COLLATE utf8mb4_bin NOT NULL,
 media_account_id BIGINT UNSIGNED NOT NULL,
 operator_id BIGINT UNSIGNED NOT NULL,
 team_id BIGINT UNSIGNED NOT NULL,
 category_id BIGINT UNSIGNED NOT NULL,
 publication_id BIGINT UNSIGNED NOT NULL,
 attempt_id BIGINT UNSIGNED NOT NULL,
 composite_output_id BIGINT UNSIGNED NOT NULL,
 title VARCHAR(4000) NOT NULL,
 account_name VARCHAR(255) NOT NULL,
 cover_resource_path VARCHAR(128) NOT NULL,
 cover_file_id BIGINT NOT NULL,
 video_sha256 CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 published_at DATETIME(6) NOT NULL,
 first_observed_at DATETIME(6) NOT NULL,
 UNIQUE KEY uq_work_identity(platform,platform_work_id),
 KEY idx_work_owner(operator_id,published_at,id),
 KEY idx_work_team(team_id,published_at,id),
 CONSTRAINT fk_work_cover FOREIGN KEY(cover_file_id) REFERENCES operations_files(id),
 CONSTRAINT fk_work_publication FOREIGN KEY(publication_id) REFERENCES publication_task(id),
 CONSTRAINT fk_work_attempt FOREIGN KEY(attempt_id) REFERENCES publication_attempt(id),
 CONSTRAINT fk_work_output FOREIGN KEY(composite_output_id) REFERENCES composite_outputs(id),
 CONSTRAINT fk_work_account FOREIGN KEY(media_account_id) REFERENCES media_accounts(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
ALTER TABLE publication_task ADD CONSTRAINT fk_publication_work FOREIGN KEY(work_id) REFERENCES work(id);
