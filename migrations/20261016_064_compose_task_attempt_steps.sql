-- One JSON document per attempt: the key steps this attempt went through,
-- appended by the worker as each phase finishes (PRD §5.8.6 — the execution
-- log is what the detail page shows, not a task-level summary). The log is
-- always read together with its attempt row and never queried by step, so it
-- lives in a column here rather than in a second table.
--
-- Nullable rather than DEFAULT (JSON_ARRAY()): the connector rows that create
-- an attempt (worker claim, manual retry) predate the column and keep their
-- INSERT statements untouched; the reader normalises NULL to an empty list.
ALTER TABLE compose_task_attempts
    ADD COLUMN steps_json JSON NULL AFTER error_message;
