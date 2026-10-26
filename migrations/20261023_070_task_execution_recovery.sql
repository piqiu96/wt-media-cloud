-- 任务运行时按用户与 attempt 收敛：先回填历史行归属，无法归属的非终态行
-- 直接以 EXECUTION_INTERRUPTED 收口。
ALTER TABLE tasks
    ADD COLUMN user_id BIGINT UNSIGNED NULL,
    ADD COLUMN attempt INT NOT NULL DEFAULT 0,
    ADD COLUMN attempt_id VARCHAR(64) NULL,
    ADD COLUMN started_at DATETIME(6) NULL,
    ADD COLUMN last_report_at DATETIME(6) NULL,
    ADD COLUMN finished_at DATETIME(6) NULL,
    ADD COLUMN error_message TEXT NULL,
    ADD COLUMN execution_timeout_seconds INT NOT NULL DEFAULT 300,
    ADD COLUMN external_side_effect TINYINT(1) NOT NULL DEFAULT 0,
    ADD COLUMN retryable TINYINT(1) NOT NULL DEFAULT 0,
    ADD COLUMN recovery_action VARCHAR(32) NULL,
    ADD COLUMN settled_by VARCHAR(16) NULL;

UPDATE tasks SET attempt = 1 WHERE attempt = 0;
UPDATE tasks SET attempt_id = CONCAT('legacy:', task_id) WHERE attempt_id IS NULL;
UPDATE tasks SET last_report_at = COALESCE(updated_at, created_at) WHERE last_report_at IS NULL;

-- 旧任务能从执行物反查归属的，尽量保留给原用户。
-- 历史 payload 里的 id 可能是非数字的外部串（如旧的 profile uuid）；这类值只跳过，
-- 否则与 BIGINT 隐式比较会在严格模式下报 1292 并让整个迁移失败。
UPDATE tasks t
    JOIN media_accounts ma
      ON JSON_UNQUOTE(JSON_EXTRACT(t.payload_json, '$.account_id')) REGEXP '^[0-9]+$'
     AND ma.id = CAST(JSON_UNQUOTE(JSON_EXTRACT(t.payload_json, '$.account_id')) AS UNSIGNED)
SET t.user_id = ma.user_id
WHERE t.user_id IS NULL;

UPDATE tasks t
    JOIN browser_profiles bp
      ON JSON_UNQUOTE(JSON_EXTRACT(t.payload_json, '$.cloud_profile_id')) REGEXP '^[0-9]+$'
     AND bp.id = CAST(JSON_UNQUOTE(JSON_EXTRACT(t.payload_json, '$.cloud_profile_id')) AS UNSIGNED)
SET t.user_id = bp.user_id
WHERE t.user_id IS NULL;

UPDATE tasks t
    JOIN browser_profiles bp
      ON JSON_UNQUOTE(JSON_EXTRACT(t.payload_json, '$.profile_id')) REGEXP '^[0-9]+$'
     AND bp.id = CAST(JSON_UNQUOTE(JSON_EXTRACT(t.payload_json, '$.profile_id')) AS UNSIGNED)
SET t.user_id = bp.user_id
WHERE t.user_id IS NULL;

UPDATE tasks
SET status = 'failed',
    error_code = 'EXECUTION_INTERRUPTED',
    error_message = '历史任务没有可归属的用户，无法继续执行',
    retryable = 1,
    recovery_action = 'retry',
    settled_by = 'legacy',
    finished_at = UTC_TIMESTAMP(6),
    last_report_at = COALESCE(last_report_at, UTC_TIMESTAMP(6)),
    updated_at = UTC_TIMESTAMP(6)
WHERE user_id IS NULL
  AND status NOT IN ('succeeded', 'failed', 'cancelled');

-- 幂等键按用户隔离，两个用户用同一个键互不影响。
ALTER TABLE tasks
    DROP INDEX uq_idempotency_key,
    ADD UNIQUE KEY uq_tasks_user_idempotency (user_id, idempotency_key),
    ADD KEY idx_tasks_user_status_created (user_id, status, created_at),
    ADD KEY idx_tasks_status_last_report (status, last_report_at);
