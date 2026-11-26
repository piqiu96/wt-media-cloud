-- Independent mutable metrics, never rewrite immutable work/publication facts.
CREATE TABLE work_metric (
 work_id BIGINT UNSIGNED NOT NULL PRIMARY KEY,
 view BIGINT UNSIGNED NULL,
 `like` BIGINT UNSIGNED NULL,
 favorite BIGINT UNSIGNED NULL,
 coin BIGINT UNSIGNED NULL,
 reply BIGINT UNSIGNED NULL,
 danmaku BIGINT UNSIGNED NULL,
 collected_at DATETIME(6) NULL,
 last_attempt_at DATETIME(6) NULL,
 status VARCHAR(16) NOT NULL,
 last_error VARCHAR(32) NOT NULL DEFAULT '',
 source VARCHAR(32) NOT NULL DEFAULT 'bilibili_public_api',
 next_attempt_at DATETIME(6) NOT NULL,
 failures INT UNSIGNED NOT NULL DEFAULT 0,
 lease_token CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
 lease_until DATETIME(6) NULL,
 KEY idx_metric_due(next_attempt_at),
 CONSTRAINT fk_metric_work FOREIGN KEY(work_id) REFERENCES work(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE work_metric_snapshot (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
 work_id BIGINT UNSIGNED NOT NULL,
 view BIGINT UNSIGNED NOT NULL,
 `like` BIGINT UNSIGNED NOT NULL,
 favorite BIGINT UNSIGNED NOT NULL,
 coin BIGINT UNSIGNED NOT NULL,
 reply BIGINT UNSIGNED NOT NULL,
 danmaku BIGINT UNSIGNED NOT NULL,
 collected_at DATETIME(6) NOT NULL,
 KEY idx_metric_snapshot(work_id,collected_at),
 CONSTRAINT fk_metric_snapshot_work FOREIGN KEY(work_id) REFERENCES work(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
