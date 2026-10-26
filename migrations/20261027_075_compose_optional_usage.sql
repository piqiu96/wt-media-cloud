-- CHG-20261026-105 (M5-F): scheduled tasks are created from the strategy team's
-- material pool, not the creator's personal working set. Such a task has no
-- material_usage relation to point at, so both the task and the output it
-- produces keep the column nullable. Human/manual tasks keep writing their
-- real usage id; existing rows are not backfilled.
ALTER TABLE compose_tasks
    MODIFY COLUMN material_usage_id BIGINT UNSIGNED NULL;

ALTER TABLE composite_outputs
    MODIFY COLUMN material_usage_id BIGINT UNSIGNED NULL;
