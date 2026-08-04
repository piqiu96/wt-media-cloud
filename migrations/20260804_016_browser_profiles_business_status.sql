ALTER TABLE browser_profiles
    ADD COLUMN business_status VARCHAR(16) NOT NULL DEFAULT 'enabled' AFTER remark;

CREATE INDEX idx_browser_profiles_business_status
    ON browser_profiles (business_status);
