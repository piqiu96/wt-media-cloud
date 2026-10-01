-- One active Desktop installation per user. Audit logs retain unbind history.
ALTER TABLE users
  ADD COLUMN device_id CHAR(64) NULL AFTER avatar_id,
  ADD COLUMN device_public_key VARBINARY(32) NULL AFTER device_id,
  ADD COLUMN device_name VARCHAR(64) NULL AFTER device_public_key,
  ADD COLUMN device_bound_at DATETIME(3) NULL AFTER device_name,
  ADD COLUMN device_last_verified_at DATETIME(3) NULL AFTER device_bound_at;
