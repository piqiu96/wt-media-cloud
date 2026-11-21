ALTER TABLE publication_task
 ADD COLUMN next_check_at DATETIME(6) NULL,
 ADD COLUMN check_generation BIGINT UNSIGNED NOT NULL DEFAULT 0,
 ADD COLUMN last_checked_at DATETIME(6) NULL,
 ADD COLUMN last_check_message VARCHAR(800) NULL,
 ADD KEY idx_publication_check_due(operator_id,status,next_check_at,id);
CREATE TABLE publication_observation (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
 publication_id BIGINT UNSIGNED NOT NULL,
 attempt_id BIGINT UNSIGNED NOT NULL,
 observer_id BIGINT UNSIGNED NOT NULL,
 generation BIGINT UNSIGNED NOT NULL,
 expected_revision BIGINT UNSIGNED NOT NULL,
 observation_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 observation_json JSON NOT NULL,
 observed_at DATETIME(6) NOT NULL,
 UNIQUE KEY uq_publication_observation(publication_id,generation),
 KEY idx_publication_observation_attempt(attempt_id,id)
);
