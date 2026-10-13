-- CHG-20261013-089 (M4-G): one row per compose-task execution attempt. The
-- task row keeps only the latest attempt's summary; this table is the record
-- the detail page reads (PRD §5.8.6 — no task summary may pose as an
-- execution log). `source` says why the attempt exists: the worker's own
-- transient retry ('auto') or a senior operator's manual retry ('manual').
-- A manual retry opens its row at authorization time (outcome 'pending',
-- started_at NULL); the worker's claim transaction stamps started_at and
-- 'running', and the terminal paths close it.
CREATE TABLE compose_task_attempts (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    task_id BIGINT UNSIGNED NOT NULL,
    attempt INT UNSIGNED NOT NULL,
    source VARCHAR(16) NOT NULL DEFAULT 'auto',
    started_at DATETIME(6) NULL,
    finished_at DATETIME(6) NULL,
    outcome VARCHAR(16) NOT NULL DEFAULT 'pending',
    stage VARCHAR(32) NOT NULL DEFAULT '',
    error_code VARCHAR(64) NOT NULL DEFAULT '',
    error_message VARCHAR(512) NOT NULL DEFAULT '',
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_compose_task_attempts_task_attempt (task_id, attempt),
    KEY idx_compose_task_attempts_task (task_id, id),
    CONSTRAINT fk_compose_task_attempts_task FOREIGN KEY (task_id) REFERENCES compose_tasks(id),
    CONSTRAINT chk_compose_task_attempts_source CHECK (source IN ('auto', 'manual')),
    CONSTRAINT chk_compose_task_attempts_outcome CHECK (outcome IN ('pending', 'running', 'success', 'failed', 'cancelled'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
