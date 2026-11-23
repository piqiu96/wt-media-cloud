ALTER TABLE publication_attempt
 ADD COLUMN paused_stage VARCHAR(32) NULL,
 ADD COLUMN last_resume_revision BIGINT UNSIGNED NULL;
