-- Only the current Cookie operation is retained; no credential version history.
ALTER TABLE media_accounts
 ADD COLUMN before_cookie LONGTEXT NULL,
 ADD COLUMN cookie_update JSON NULL;
ALTER TABLE browser_profiles ADD COLUMN cookie_operation_id VARCHAR(64) NULL;
