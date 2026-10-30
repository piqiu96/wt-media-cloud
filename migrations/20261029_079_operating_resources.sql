-- CHG-20261028-099: converge the three operating-resource domains.
-- This migration intentionally renames and removes transitional fields instead
-- of keeping dual-read/dual-write compatibility.

DROP TABLE IF EXISTS sensitive_profile_permits;
DROP TABLE IF EXISTS sensitive_browser_tasks;
DROP TABLE IF EXISTS tasks;

-- The category migration created a second account FK while renaming the table.
ALTER TABLE media_account_categories DROP FOREIGN KEY fk_media_account_games_account;

ALTER TABLE media_accounts
    DROP CHECK chk_media_accounts_business_status,
    DROP CHECK chk_media_accounts_login_status,
    DROP INDEX idx_media_accounts_status,
    DROP INDEX idx_media_accounts_business_login;

UPDATE media_accounts SET business_status = IF(business_status = 'enabled', '1', '0');

ALTER TABLE media_accounts
    CHANGE COLUMN name platform_name VARCHAR(255) NULL,
    CHANGE COLUMN avatar_url platform_avatar_url VARCHAR(2048) NULL,
    CHANGE COLUMN login_status platform_login_status VARCHAR(32) NOT NULL DEFAULT 'unknown',
    CHANGE COLUMN business_status enabled TINYINT(1) NOT NULL DEFAULT 1,
    CHANGE COLUMN last_checked_at last_check_at DATETIME(6) NULL,
    CHANGE COLUMN check_items last_check_items JSON NULL,
    CHANGE COLUMN active_cookie_updated_at cookie_updated_at DATETIME(6) NULL,
    ADD KEY idx_media_accounts_enabled_identification (enabled, identification_status),
    ADD KEY idx_media_accounts_enabled_login (enabled, platform_login_status),
    ADD CONSTRAINT chk_media_accounts_enabled CHECK (enabled IN (0, 1)),
    ADD CONSTRAINT chk_media_accounts_platform_login_status CHECK (platform_login_status IN ('unknown', 'normal', 'not_logged_in', 'verification_needed', 'expired', 'restricted', 'account_mismatch', 'environment_error'));

CREATE TABLE media_account_tags_new (
    media_account_id BIGINT UNSIGNED NOT NULL,
    tag_name VARCHAR(64) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (media_account_id, tag_name),
    KEY idx_media_account_tags_name (tag_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO media_account_tags_new (media_account_id, tag_name, created_at)
SELECT media_account_id, tag_name, MIN(created_at)
FROM media_account_tags
GROUP BY media_account_id, tag_name;

DROP TABLE media_account_tags;
RENAME TABLE media_account_tags_new TO media_account_tags;
ALTER TABLE media_account_tags
    ADD CONSTRAINT fk_media_account_tags_account FOREIGN KEY (media_account_id) REFERENCES media_accounts(id) ON DELETE CASCADE;
RENAME TABLE account_groups TO media_account_groups;

RENAME TABLE proxy_configs TO proxies;

ALTER TABLE proxies
    DROP FOREIGN KEY fk_proxy_configs_team,
    DROP INDEX idx_proxy_configs_status,
    DROP INDEX idx_proxy_configs_team,
    CHANGE COLUMN source_type source VARCHAR(16) NOT NULL DEFAULT 'static',
    CHANGE COLUMN proxy_protocol protocol VARCHAR(16) NOT NULL DEFAULT 'http',
    CHANGE COLUMN tags_json tags JSON NULL,
    CHANGE COLUMN max_profile_count max_profiles INT NOT NULL DEFAULT 3,
    ADD COLUMN enabled TINYINT(1) NOT NULL DEFAULT 1 AFTER expires_at,
    ADD COLUMN last_check_message VARCHAR(512) NOT NULL DEFAULT '' AFTER last_check_result;

UPDATE proxies SET enabled = IF(business_status = 'active', 1, 0);

ALTER TABLE proxies
    DROP COLUMN business_status,
    DROP COLUMN observed_exit_ip,
    ADD KEY idx_proxies_team_enabled (team_id, enabled),
    ADD KEY idx_proxies_supplier (supplier),
    ADD CONSTRAINT chk_proxies_enabled CHECK (enabled IN (0, 1)),
    ADD CONSTRAINT chk_proxies_protocol CHECK (protocol IN ('http', 'https', 'socks5')),
    ADD CONSTRAINT chk_proxies_source CHECK (source IN ('static', 'api')),
    ADD CONSTRAINT fk_proxies_team FOREIGN KEY (team_id) REFERENCES operation_teams(id);

ALTER TABLE browser_profiles ADD KEY idx_browser_profiles_user (user_id);

ALTER TABLE browser_profiles
    DROP FOREIGN KEY fk_browser_profiles_proxy,
    DROP CHECK chk_browser_profiles_local_status,
    DROP INDEX uq_browser_profiles_user_main_bit,
    DROP INDEX idx_browser_profiles_main_user,
    DROP INDEX idx_browser_profiles_user_status,
    DROP INDEX idx_browser_profiles_business_status,
    DROP INDEX idx_browser_profiles_proxy_id,
    DROP INDEX idx_browser_profiles_user_main;

UPDATE browser_profiles SET business_status = IF(business_status = 'enabled', '1', '0');

ALTER TABLE browser_profiles
    CHANGE COLUMN main_user_id bit_main_user_id VARCHAR(255) NOT NULL,
    CHANGE COLUMN profile_user_id bit_user_id VARCHAR(255) NOT NULL,
    CHANGE COLUMN name bit_name VARCHAR(255) NOT NULL DEFAULT '',
    CHANGE COLUMN seq bit_seq INT NOT NULL DEFAULT 0,
    CHANGE COLUMN group_id bit_group_id VARCHAR(255) NULL,
    CHANGE COLUMN group_name bit_group_name VARCHAR(255) NULL,
    CHANGE COLUMN bit_status bit_profile_status VARCHAR(64) NULL,
    CHANGE COLUMN proxy_type bit_proxy_type VARCHAR(32) NULL,
    CHANGE COLUMN proxy_host bit_proxy_host VARCHAR(255) NULL,
    CHANGE COLUMN proxy_port bit_proxy_port INT NOT NULL DEFAULT 0,
    CHANGE COLUMN remark bit_remark TEXT NULL,
    CHANGE COLUMN cloud_remark remark VARCHAR(500) NOT NULL DEFAULT '',
    CHANGE COLUMN local_status local_state VARCHAR(32) NOT NULL,
    CHANGE COLUMN business_status enabled TINYINT(1) NOT NULL DEFAULT 1,
    ADD COLUMN reserved_proxy_id VARCHAR(64) NULL AFTER proxy_id,
    ADD COLUMN reserved_since DATETIME(6) NULL AFTER reserved_proxy_id,
    ADD UNIQUE KEY uq_browser_profiles_user_bit (user_id, bit_main_user_id, bit_profile_id),
    ADD KEY idx_browser_profiles_user_main (user_id, bit_main_user_id),
    ADD KEY idx_browser_profiles_user_state (user_id, local_state),
    ADD KEY idx_browser_profiles_main_state (bit_main_user_id, local_state),
    ADD KEY idx_browser_profiles_enabled (enabled),
    ADD KEY idx_browser_profiles_proxy (proxy_id),
    ADD KEY idx_browser_profiles_reserved_proxy (reserved_proxy_id),
    ADD CONSTRAINT chk_browser_profiles_local_state CHECK (local_state IN ('active', 'local_missing', 'archived')),
    ADD CONSTRAINT chk_browser_profiles_enabled CHECK (enabled IN (0, 1)),
    ADD CONSTRAINT fk_browser_profiles_proxy FOREIGN KEY (proxy_id) REFERENCES proxies(id) ON DELETE RESTRICT,
    ADD CONSTRAINT fk_browser_profiles_reserved_proxy FOREIGN KEY (reserved_proxy_id) REFERENCES proxies(id) ON DELETE RESTRICT;

DROP TABLE profile_sync_candidates;
DROP TABLE profile_sync_scans;

CREATE TABLE browser_profile_syncs (
    id VARCHAR(64) NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    bit_main_user_id VARCHAR(255) NOT NULL,
    snapshot JSON NOT NULL,
    observed_at DATETIME(6) NOT NULL,
    expires_at DATETIME(6) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_browser_profile_syncs_user (user_id),
    CONSTRAINT fk_browser_profile_syncs_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
