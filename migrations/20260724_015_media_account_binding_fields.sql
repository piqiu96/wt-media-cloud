ALTER TABLE media_accounts
    ADD COLUMN remark VARCHAR(500) NULL AFTER browser_profile_id;

CREATE INDEX idx_media_accounts_business_login
    ON media_accounts (business_status, login_status);
