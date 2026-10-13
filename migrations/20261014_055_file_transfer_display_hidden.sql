-- A user's download-centre history may be hidden without changing transfer or asset facts.
ALTER TABLE file_transfer_tasks ADD COLUMN hidden_at DATETIME(6) NULL AFTER finished_at;
