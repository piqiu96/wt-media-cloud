-- Successful provider capture time. Historical values are unknown, not migration time.
ALTER TABLE source_contents ADD COLUMN metrics_updated_at DATETIME(6) NULL AFTER updated_at;
