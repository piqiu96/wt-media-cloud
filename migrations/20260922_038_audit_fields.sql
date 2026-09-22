-- Operator / audit fields captured at the time of the operation.
ALTER TABLE discovery_strategies ADD COLUMN updated_by BIGINT UNSIGNED NULL AFTER created_by;
ALTER TABLE crawl_tasks ADD COLUMN updated_by BIGINT UNSIGNED NULL AFTER created_by;
ALTER TABLE source_contents ADD COLUMN updated_by BIGINT UNSIGNED NULL AFTER created_by;
ALTER TABLE source_contents ADD COLUMN audited_by BIGINT UNSIGNED NULL AFTER updated_by;
ALTER TABLE source_contents ADD COLUMN audited_at DATETIME(6) NULL AFTER audited_by;
