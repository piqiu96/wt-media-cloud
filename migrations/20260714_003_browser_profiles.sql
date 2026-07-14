ALTER TABLE users
    ADD COLUMN bit_main_user_id VARCHAR(255) NULL,
    ADD COLUMN bit_account_status VARCHAR(32) NULL,
    ADD COLUMN bit_account_bound_at DATETIME(6) NULL,
    ADD COLUMN bit_account_last_verified_at DATETIME(6) NULL,
    ADD KEY idx_users_bit_main_user_id (bit_main_user_id);

CREATE TABLE browser_profiles (
    id VARCHAR(64) NOT NULL,
    user_id VARCHAR(64) NOT NULL,
    bit_profile_id VARCHAR(255) NOT NULL,
    main_user_id VARCHAR(255) NOT NULL,
    profile_user_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL DEFAULT '',
    seq INT NOT NULL DEFAULT 0,
    group_id VARCHAR(255) NULL,
    group_name VARCHAR(255) NULL,
    bit_status VARCHAR(64) NULL,
    bit_updated_at VARCHAR(64) NULL,
    local_status VARCHAR(32) NOT NULL,
    last_synced_at DATETIME(6) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_browser_profiles_user_bit_profile (user_id, bit_profile_id),
    UNIQUE KEY uq_browser_profiles_bit_profile (bit_profile_id),
    KEY idx_browser_profiles_user_status (user_id, local_status),
    KEY idx_browser_profiles_main_user (main_user_id, local_status),
    CONSTRAINT fk_browser_profiles_user FOREIGN KEY (user_id) REFERENCES users (id),
    CONSTRAINT chk_browser_profiles_local_status CHECK (local_status IN ('active', 'local_missing', 'archived'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE profile_sync_scans (
    id VARCHAR(64) NOT NULL,
    user_id VARCHAR(64) NOT NULL,
    main_user_id VARCHAR(255) NOT NULL,
    status VARCHAR(32) NOT NULL,
    diff_json JSON NOT NULL,
    created_at DATETIME(6) NOT NULL,
    expires_at DATETIME(6) NOT NULL,
    confirmed_at DATETIME(6) NULL,
    PRIMARY KEY (id),
    KEY idx_profile_sync_scans_user_created (user_id, created_at),
    CONSTRAINT fk_profile_sync_scans_user FOREIGN KEY (user_id) REFERENCES users (id),
    CONSTRAINT chk_profile_sync_scans_status CHECK (status IN ('ready', 'confirmed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE profile_sync_candidates (
    scan_id VARCHAR(64) NOT NULL,
    profile_id VARCHAR(64) NOT NULL,
    bit_profile_id VARCHAR(255) NOT NULL,
    main_user_id VARCHAR(255) NOT NULL,
    profile_user_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL DEFAULT '',
    seq INT NOT NULL DEFAULT 0,
    group_id VARCHAR(255) NULL,
    group_name VARCHAR(255) NULL,
    bit_status VARCHAR(64) NULL,
    bit_updated_at VARCHAR(64) NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (scan_id, bit_profile_id),
    CONSTRAINT fk_profile_sync_candidates_scan FOREIGN KEY (scan_id) REFERENCES profile_sync_scans (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

ALTER TABLE media_accounts
    ADD CONSTRAINT fk_media_accounts_browser_profile FOREIGN KEY (browser_profile_id) REFERENCES browser_profiles (id);
