CREATE TABLE IF NOT EXISTS publication_batch (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
 team_id BIGINT UNSIGNED NOT NULL,
 operator_id BIGINT UNSIGNED NOT NULL,
 created_by BIGINT UNSIGNED NOT NULL,
 name VARCHAR(100) NOT NULL,
 platform VARCHAR(32) NOT NULL,
 targets_json JSON NOT NULL,
 config_json JSON NOT NULL,
 readiness_json JSON NOT NULL,
 candidates_count INT UNSIGNED NOT NULL,
 concurrency_limit INT UNSIGNED NOT NULL,
 group_interval_seconds INT UNSIGNED NOT NULL,
 between_group_policy VARCHAR(32) NOT NULL,
 exception_task_limit INT UNSIGNED NOT NULL,
 configuration_status VARCHAR(24) NOT NULL,
 execution_status VARCHAR(24) NOT NULL DEFAULT 'not_started',
 started_at DATETIME(6) NULL,
 revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
 request_key VARCHAR(64) COLLATE utf8mb4_bin NOT NULL,
 request_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 created_at DATETIME(6) NOT NULL,
 updated_at DATETIME(6) NOT NULL,
 UNIQUE KEY uq_publication_batch_request(created_by,request_key),
 KEY idx_publication_batch_team(team_id,created_at,id),
 KEY idx_publication_batch_operator(operator_id,created_at,id),
 CONSTRAINT chk_publication_batch_configuration CHECK(configuration_status IN ('incomplete','ready')),
 CONSTRAINT chk_publication_batch_execution CHECK(execution_status IN ('not_started','running','paused','ended','cancelled'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS publication_batch_file (
 batch_id BIGINT UNSIGNED NOT NULL,
 file_id BIGINT NOT NULL,
 PRIMARY KEY(batch_id,file_id),
 KEY idx_publication_batch_file(file_id,batch_id),
 CONSTRAINT fk_publication_batch_file_batch FOREIGN KEY(batch_id) REFERENCES publication_batch(id) ON DELETE CASCADE,
 CONSTRAINT fk_publication_batch_file_file FOREIGN KEY(file_id) REFERENCES operations_files(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
