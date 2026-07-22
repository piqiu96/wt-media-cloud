ALTER TABLE profile_sync_candidates
    ADD COLUMN proxy_type VARCHAR(32) NULL AFTER bit_updated_at,
    ADD COLUMN proxy_host VARCHAR(255) NULL AFTER proxy_type,
    ADD COLUMN proxy_port INT NOT NULL DEFAULT 0 AFTER proxy_host,
    ADD COLUMN remark TEXT NULL AFTER proxy_port;
