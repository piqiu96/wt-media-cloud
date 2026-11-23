ALTER TABLE publication_task
 ADD COLUMN check_not_found_count INT UNSIGNED NOT NULL DEFAULT 0,
 ADD COLUMN check_stopped_at DATETIME(6) NULL,
 ADD COLUMN check_browser_used_at DATETIME(6) NULL;
