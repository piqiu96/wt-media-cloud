-- Three reads take rows of compose_tasks by (strategy, material): the create
-- drawer's per-material aggregate, the create guard's in-flight check (a
-- locking read, so it also takes locks on the range it scans), and the
-- scheduler's per-candidate check. The existing keys do not serve that pair —
-- idx_compose_tasks_material spans every strategy, and
-- idx_compose_tasks_scope_status leads with the team.
--
-- Deliberately not UNIQUE: the same pair is allowed to run more than once, and
-- 再生成一版 depends on it. The guard against a duplicate in flight is the
-- create transaction, not a constraint.
ALTER TABLE compose_tasks
    ADD KEY idx_compose_tasks_strategy_material (compose_strategy_id, material_id);
