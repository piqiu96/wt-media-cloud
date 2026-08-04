-- Migrate browser_profiles.id from varchar(64) string to BIGINT UNSIGNED AUTO_INCREMENT
-- primary key (Milestone M2-B L206: system record ID uses DB auto-increment primary key).
-- Also add cloud_remark for the two-field remark model (BitBrowser remark + Cloud remark).

-- 1. Drop FKs referencing browser_profiles.id
ALTER TABLE media_accounts DROP FOREIGN KEY fk_media_accounts_browser_profile;
ALTER TABLE browser_profile_runtime_presence DROP FOREIGN KEY fk_browser_profile_runtime_presence_profile;
ALTER TABLE sensitive_browser_tasks DROP FOREIGN KEY fk_sensitive_browser_task_profile;
ALTER TABLE sensitive_profile_permits DROP FOREIGN KEY fk_sensitive_profile_permit_profile;

-- 2. Convert browser_profiles.id: drop old string PK, keep legacy string id, add auto-increment PK
ALTER TABLE browser_profiles DROP PRIMARY KEY;
ALTER TABLE browser_profiles CHANGE COLUMN id legacy_id VARCHAR(64) NULL;
ALTER TABLE browser_profiles
    ADD COLUMN id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT FIRST,
    ADD PRIMARY KEY (id);

-- 3. Add temporary numeric FK columns to child tables
ALTER TABLE media_accounts ADD COLUMN browser_profile_id_new BIGINT UNSIGNED NULL AFTER browser_profile_id;
ALTER TABLE browser_profile_runtime_presence ADD COLUMN profile_id_new BIGINT UNSIGNED NULL AFTER profile_id;
ALTER TABLE sensitive_browser_tasks ADD COLUMN profile_id_new BIGINT UNSIGNED NULL AFTER profile_id;
ALTER TABLE sensitive_profile_permits ADD COLUMN profile_id_new BIGINT UNSIGNED NULL AFTER profile_id;

-- 4. Migrate data: map string legacy_id -> numeric id
UPDATE media_accounts ma JOIN browser_profiles bp ON bp.legacy_id = ma.browser_profile_id
    SET ma.browser_profile_id_new = bp.id;
UPDATE browser_profile_runtime_presence rp JOIN browser_profiles bp ON bp.legacy_id = rp.profile_id
    SET rp.profile_id_new = bp.id;
UPDATE sensitive_browser_tasks st JOIN browser_profiles bp ON bp.legacy_id = st.profile_id
    SET st.profile_id_new = bp.id;
UPDATE sensitive_profile_permits sp JOIN browser_profiles bp ON bp.legacy_id = sp.profile_id
    SET sp.profile_id_new = bp.id;

-- 5. Drop old string FK columns (and their indexes first) then rename new numeric columns
ALTER TABLE media_accounts DROP INDEX uq_media_accounts_profile_platform;
ALTER TABLE media_accounts DROP COLUMN browser_profile_id,
    CHANGE COLUMN browser_profile_id_new browser_profile_id BIGINT UNSIGNED NULL;
ALTER TABLE browser_profile_runtime_presence DROP INDEX uq_browser_profile_runtime_presence_profile;
ALTER TABLE browser_profile_runtime_presence DROP COLUMN profile_id,
    CHANGE COLUMN profile_id_new profile_id BIGINT UNSIGNED NULL;
ALTER TABLE sensitive_browser_tasks DROP INDEX idx_sensitive_browser_tasks_profile_status;
ALTER TABLE sensitive_browser_tasks DROP COLUMN profile_id,
    CHANGE COLUMN profile_id_new profile_id BIGINT UNSIGNED NULL;
ALTER TABLE sensitive_profile_permits DROP INDEX idx_sensitive_profile_permit_profile_time;
ALTER TABLE sensitive_profile_permits DROP COLUMN profile_id,
    CHANGE COLUMN profile_id_new profile_id BIGINT UNSIGNED NULL;

-- 6. Re-add indexes on the new numeric FK columns
ALTER TABLE media_accounts
    ADD UNIQUE INDEX uq_media_accounts_profile_platform (browser_profile_id, platform);
ALTER TABLE browser_profile_runtime_presence
    ADD UNIQUE INDEX uq_browser_profile_runtime_presence_profile (profile_id);
ALTER TABLE sensitive_browser_tasks
    ADD INDEX idx_sensitive_browser_tasks_profile_status (profile_id, status, created_at);
ALTER TABLE sensitive_profile_permits
    ADD INDEX idx_sensitive_profile_permit_profile_time (profile_id, acquired_at);

-- 7. Re-add FKs
ALTER TABLE media_accounts
    ADD CONSTRAINT fk_media_accounts_browser_profile FOREIGN KEY (browser_profile_id) REFERENCES browser_profiles(id);
ALTER TABLE browser_profile_runtime_presence
    ADD CONSTRAINT fk_browser_profile_runtime_presence_profile FOREIGN KEY (profile_id) REFERENCES browser_profiles(id);
ALTER TABLE sensitive_browser_tasks
    ADD CONSTRAINT fk_sensitive_browser_task_profile FOREIGN KEY (profile_id) REFERENCES browser_profiles(id);
ALTER TABLE sensitive_profile_permits
    ADD CONSTRAINT fk_sensitive_profile_permit_profile FOREIGN KEY (profile_id) REFERENCES browser_profiles(id);

-- 8. Drop the legacy string id column
ALTER TABLE browser_profiles DROP COLUMN legacy_id;

-- 9. Add cloud_remark for the two-field remark model
ALTER TABLE browser_profiles ADD COLUMN cloud_remark VARCHAR(500) NOT NULL DEFAULT '' AFTER remark;
