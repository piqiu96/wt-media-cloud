-- CHG-20261015-096 (M5-C5): retry eligibility must survive Worker restarts and
-- be enforced by every claimant, rather than only by a local poll interval.
ALTER TABLE compose_tasks
    ADD COLUMN next_attempt_at DATETIME(6) NULL AFTER max_attempts,
    ADD KEY idx_compose_tasks_retry_claim (status, next_attempt_at, id);
