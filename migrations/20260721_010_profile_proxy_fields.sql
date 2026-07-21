ALTER TABLE browser_profiles
    ADD COLUMN proxy_type VARCHAR(32) NULL,
    ADD COLUMN proxy_host VARCHAR(255) NULL,
    ADD COLUMN proxy_port INT NOT NULL DEFAULT 0,
    ADD COLUMN remark TEXT NULL;
