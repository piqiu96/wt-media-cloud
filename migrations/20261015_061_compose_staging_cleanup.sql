-- CHG-20261015-096 (M5-C5): closed attempts can have a staged object left
-- behind when deletion fails or a worker exits between commit and cleanup.
ALTER TABLE compose_task_attempts
    ADD COLUMN staging_cleaned_at DATETIME(6) NULL AFTER error_message,
    ADD KEY idx_compose_attempts_staging_cleanup (staging_cleaned_at, outcome, id);

-- Older attempts used a task-level staging key. Do not infer their attempt
-- key or let a new scanner delete a currently active legacy key.
UPDATE compose_task_attempts SET staging_cleaned_at = UTC_TIMESTAMP(6);
