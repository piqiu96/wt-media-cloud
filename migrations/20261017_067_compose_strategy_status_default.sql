-- 066 renamed the allowed strategy statuses but left the column default at the
-- old 'active', which chk_compose_strategies_status now rejects: any INSERT that
-- omits status fails instead of landing on a valid value. Point the fallback at
-- 'disabled' so a row that does not state its status cannot start producing
-- tasks by accident.
ALTER TABLE compose_strategies
    MODIFY COLUMN status VARCHAR(16) NOT NULL DEFAULT 'disabled';
