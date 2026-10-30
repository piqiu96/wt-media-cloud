-- Keep only the current/last proxy operation, without credentials or history.
ALTER TABLE browser_profiles ADD COLUMN proxy_operation JSON NULL AFTER reserved_since;
