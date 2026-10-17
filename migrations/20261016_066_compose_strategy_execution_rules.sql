-- Strategy status and execution mode are separate facts. Existing rows start in
-- manual mode so a legacy schedule cannot run before its rule groups are set.
ALTER TABLE compose_strategies DROP CHECK chk_compose_strategies_status;

UPDATE compose_strategies
SET status = CASE status WHEN 'active' THEN 'enabled' ELSE 'disabled' END,
    next_run_at = NULL;

ALTER TABLE compose_strategies
    ADD COLUMN execution_mode VARCHAR(16) NOT NULL DEFAULT 'manual' AFTER config_json,
    ADD COLUMN priority_rules_json JSON NULL AFTER schedule_json,
    ADD COLUMN single_limit INT UNSIGNED NOT NULL DEFAULT 1 AFTER priority_rules_json,
    ADD COLUMN daily_limit INT UNSIGNED NOT NULL DEFAULT 1 AFTER single_limit,
    ADD CONSTRAINT chk_compose_strategies_status CHECK (status IN ('enabled', 'disabled')),
    ADD CONSTRAINT chk_compose_strategies_execution_mode CHECK (execution_mode IN ('manual', 'scheduled')),
    ADD CONSTRAINT chk_compose_strategies_limits CHECK (single_limit BETWEEN 1 AND 100 AND daily_limit BETWEEN single_limit AND 1000);

UPDATE compose_strategies SET priority_rules_json = JSON_ARRAY();

ALTER TABLE compose_strategies MODIFY COLUMN priority_rules_json JSON NOT NULL;
