-- System avatar choice. Existing nickname column was added in migration 037.
ALTER TABLE users ADD COLUMN avatar_id VARCHAR(24) NOT NULL DEFAULT 'sky' AFTER nickname;
