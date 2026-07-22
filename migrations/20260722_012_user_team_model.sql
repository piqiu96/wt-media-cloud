CREATE TABLE operation_teams (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    name VARCHAR(64) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_operation_teams_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO operation_teams (name, created_at, updated_at)
SELECT '迁移默认分组', UTC_TIMESTAMP(6), UTC_TIMESTAMP(6)
WHERE EXISTS (SELECT 1 FROM users WHERE role <> 'technician');

ALTER TABLE users
    DROP CHECK chk_users_role,
    ADD COLUMN uid BIGINT UNSIGNED NOT NULL AUTO_INCREMENT UNIQUE,
    ADD COLUMN team_id BIGINT UNSIGNED NULL;

UPDATE users SET role = 'admin' WHERE role = 'technician';

UPDATE users
SET team_id = (SELECT id FROM operation_teams WHERE name = '迁移默认分组')
WHERE role IN ('operator', 'senior_operator');

ALTER TABLE user_game_scopes ADD COLUMN user_uid BIGINT UNSIGNED NULL;
ALTER TABLE user_sessions ADD COLUMN user_uid BIGINT UNSIGNED NULL;
ALTER TABLE audit_logs ADD COLUMN actor_user_uid BIGINT UNSIGNED NULL;
ALTER TABLE media_accounts
    ADD COLUMN user_uid BIGINT UNSIGNED NULL,
    ADD COLUMN team_id BIGINT UNSIGNED NULL;
ALTER TABLE media_account_tags ADD COLUMN user_uid BIGINT UNSIGNED NULL;
ALTER TABLE browser_profiles
    ADD COLUMN user_uid BIGINT UNSIGNED NULL,
    ADD COLUMN team_id BIGINT UNSIGNED NULL;
ALTER TABLE profile_sync_scans
    ADD COLUMN user_uid BIGINT UNSIGNED NULL,
    ADD COLUMN team_id BIGINT UNSIGNED NULL;
ALTER TABLE local_agent_binding_tickets ADD COLUMN user_uid BIGINT UNSIGNED NULL;
ALTER TABLE local_agent_nodes ADD COLUMN user_uid BIGINT UNSIGNED NULL;
ALTER TABLE browser_profile_runtime_presence ADD COLUMN user_uid BIGINT UNSIGNED NULL;
ALTER TABLE sensitive_browser_tasks ADD COLUMN user_uid BIGINT UNSIGNED NULL;
ALTER TABLE sensitive_profile_permits ADD COLUMN user_uid BIGINT UNSIGNED NULL;

UPDATE user_game_scopes child JOIN users parent ON child.user_id = parent.id SET child.user_uid = parent.uid;
UPDATE user_sessions child JOIN users parent ON child.user_id = parent.id SET child.user_uid = parent.uid;
UPDATE audit_logs child JOIN users parent ON child.actor_user_id = parent.id SET child.actor_user_uid = parent.uid;
UPDATE media_accounts child JOIN users parent ON child.user_id = parent.id SET child.user_uid = parent.uid, child.team_id = parent.team_id;
UPDATE media_account_tags child JOIN users parent ON child.user_id = parent.id SET child.user_uid = parent.uid;
UPDATE browser_profiles child JOIN users parent ON child.user_id = parent.id SET child.user_uid = parent.uid, child.team_id = parent.team_id;
UPDATE profile_sync_scans child JOIN users parent ON child.user_id = parent.id SET child.user_uid = parent.uid, child.team_id = parent.team_id;
UPDATE local_agent_binding_tickets child JOIN users parent ON child.user_id = parent.id SET child.user_uid = parent.uid;
UPDATE local_agent_nodes child JOIN users parent ON child.user_id = parent.id SET child.user_uid = parent.uid;
UPDATE browser_profile_runtime_presence child JOIN users parent ON child.user_id = parent.id SET child.user_uid = parent.uid;
UPDATE sensitive_browser_tasks child JOIN users parent ON child.user_id = parent.id SET child.user_uid = parent.uid;
UPDATE sensitive_profile_permits child JOIN users parent ON child.user_id = parent.id SET child.user_uid = parent.uid;

CREATE TEMPORARY TABLE m2_a1_migration_guard (
    valid TINYINT NOT NULL,
    CONSTRAINT chk_m2_a1_migration_guard CHECK (valid = 1)
);

INSERT INTO m2_a1_migration_guard (valid)
SELECT IF(
    (SELECT COUNT(*) FROM users WHERE uid IS NULL) = 0
    AND (SELECT COUNT(*) FROM users WHERE role = 'admin' AND team_id IS NOT NULL) = 0
    AND (SELECT COUNT(*) FROM users WHERE role IN ('operator', 'senior_operator') AND team_id IS NULL) = 0
    AND (SELECT COUNT(*) FROM user_game_scopes WHERE user_uid IS NULL) = 0
    AND (SELECT COUNT(*) FROM user_sessions WHERE user_uid IS NULL) = 0
    AND (SELECT COUNT(*) FROM media_accounts WHERE user_uid IS NULL) = 0
    AND (SELECT COUNT(*) FROM media_account_tags WHERE user_uid IS NULL) = 0
    AND (SELECT COUNT(*) FROM browser_profiles WHERE user_uid IS NULL) = 0
    AND (SELECT COUNT(*) FROM profile_sync_scans WHERE user_uid IS NULL) = 0
    AND (SELECT COUNT(*) FROM local_agent_binding_tickets WHERE user_uid IS NULL) = 0
    AND (SELECT COUNT(*) FROM local_agent_nodes WHERE user_uid IS NULL) = 0
    AND (SELECT COUNT(*) FROM browser_profile_runtime_presence WHERE user_uid IS NULL) = 0
    AND (SELECT COUNT(*) FROM sensitive_browser_tasks WHERE user_uid IS NULL) = 0
    AND (SELECT COUNT(*) FROM sensitive_profile_permits WHERE user_uid IS NULL) = 0,
    1,
    0
);

DROP TEMPORARY TABLE m2_a1_migration_guard;

ALTER TABLE user_game_scopes DROP FOREIGN KEY fk_user_game_scopes_user, DROP PRIMARY KEY;
ALTER TABLE user_sessions DROP FOREIGN KEY fk_user_sessions_user, DROP INDEX idx_user_sessions_active_user;
ALTER TABLE media_accounts
    DROP FOREIGN KEY fk_media_accounts_user,
    DROP INDEX uq_media_accounts_user_platform_identity,
    DROP INDEX idx_media_accounts_user_game;
ALTER TABLE media_account_tags
    DROP FOREIGN KEY fk_media_account_tags_user,
    DROP INDEX uq_media_account_tags_owner_account_name,
    DROP INDEX idx_media_account_tags_name;
ALTER TABLE browser_profiles
    DROP FOREIGN KEY fk_browser_profiles_user,
    DROP INDEX uq_browser_profiles_user_bit_profile,
    DROP INDEX idx_browser_profiles_user_status;
ALTER TABLE profile_sync_scans DROP FOREIGN KEY fk_profile_sync_scans_user, DROP INDEX idx_profile_sync_scans_user_created;
ALTER TABLE local_agent_binding_tickets DROP FOREIGN KEY fk_local_agent_binding_ticket_user;
ALTER TABLE local_agent_nodes
    DROP FOREIGN KEY fk_local_agent_node_user,
    DROP INDEX idx_local_agent_node_user_status,
    DROP INDEX idx_local_agent_node_device;
ALTER TABLE browser_profile_runtime_presence
    DROP FOREIGN KEY fk_browser_profile_runtime_presence_user,
    DROP INDEX idx_browser_profile_runtime_presence_user;
ALTER TABLE sensitive_browser_tasks DROP FOREIGN KEY fk_sensitive_browser_task_user;
ALTER TABLE sensitive_profile_permits DROP FOREIGN KEY fk_sensitive_profile_permit_user;

ALTER TABLE user_game_scopes
    DROP COLUMN user_id,
    CHANGE COLUMN user_uid user_id BIGINT UNSIGNED NOT NULL,
    ADD PRIMARY KEY (user_id, game_id);

ALTER TABLE user_sessions
    DROP COLUMN user_id,
    CHANGE COLUMN user_uid user_id BIGINT UNSIGNED NOT NULL,
    ADD KEY idx_user_sessions_active_user (user_id, invalidated_at);

ALTER TABLE audit_logs
    DROP INDEX idx_audit_logs_actor_created,
    DROP COLUMN actor_user_id,
    CHANGE COLUMN actor_user_uid actor_user_id BIGINT UNSIGNED NULL,
    ADD KEY idx_audit_logs_actor_created (actor_user_id, created_at);

ALTER TABLE media_accounts
    DROP COLUMN user_id,
    CHANGE COLUMN user_uid user_id BIGINT UNSIGNED NOT NULL,
    ADD UNIQUE KEY uq_media_accounts_user_platform_identity (user_id, platform, platform_account_id),
    ADD KEY idx_media_accounts_user_game (user_id, game_id);

ALTER TABLE media_account_tags
    DROP COLUMN user_id,
    CHANGE COLUMN user_uid user_id BIGINT UNSIGNED NOT NULL,
    ADD UNIQUE KEY uq_media_account_tags_owner_account_name (user_id, media_account_id, tag_name),
    ADD KEY idx_media_account_tags_name (user_id, tag_name, media_account_id);

ALTER TABLE browser_profiles
    DROP COLUMN user_id,
    CHANGE COLUMN user_uid user_id BIGINT UNSIGNED NOT NULL,
    ADD UNIQUE KEY uq_browser_profiles_user_bit_profile (user_id, bit_profile_id),
    ADD KEY idx_browser_profiles_user_status (user_id, local_status);

ALTER TABLE profile_sync_scans
    DROP COLUMN user_id,
    CHANGE COLUMN user_uid user_id BIGINT UNSIGNED NOT NULL,
    ADD KEY idx_profile_sync_scans_user_created (user_id, created_at);

ALTER TABLE local_agent_binding_tickets
    DROP COLUMN user_id,
    CHANGE COLUMN user_uid user_id BIGINT UNSIGNED NOT NULL;

ALTER TABLE local_agent_nodes
    DROP COLUMN user_id,
    CHANGE COLUMN user_uid user_id BIGINT UNSIGNED NOT NULL,
    ADD KEY idx_local_agent_node_user_status (user_id, status, last_heartbeat_at),
    ADD KEY idx_local_agent_node_device (user_id, device_id, registered_at);

ALTER TABLE browser_profile_runtime_presence
    DROP COLUMN user_id,
    CHANGE COLUMN user_uid user_id BIGINT UNSIGNED NOT NULL,
    ADD KEY idx_browser_profile_runtime_presence_user (user_id, status, last_seen_at);

ALTER TABLE sensitive_browser_tasks
    DROP COLUMN user_id,
    CHANGE COLUMN user_uid user_id BIGINT UNSIGNED NOT NULL;

ALTER TABLE sensitive_profile_permits
    DROP COLUMN user_id,
    CHANGE COLUMN user_uid user_id BIGINT UNSIGNED NOT NULL;

ALTER TABLE users
    DROP PRIMARY KEY,
    CHANGE COLUMN id legacy_id VARCHAR(64) NULL,
    CHANGE COLUMN uid id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    ADD PRIMARY KEY (id),
    ADD UNIQUE KEY uq_users_legacy_id (legacy_id),
    ADD KEY idx_users_team_role_status (team_id, role, status),
    ADD CONSTRAINT chk_users_role CHECK (role IN ('operator', 'senior_operator', 'admin')),
    ADD CONSTRAINT chk_users_role_team CHECK ((role = 'admin' AND team_id IS NULL) OR (role IN ('operator', 'senior_operator') AND team_id IS NOT NULL)),
    ADD CONSTRAINT fk_users_team FOREIGN KEY (team_id) REFERENCES operation_teams(id);

ALTER TABLE user_game_scopes ADD CONSTRAINT fk_user_game_scopes_user FOREIGN KEY (user_id) REFERENCES users(id);
ALTER TABLE user_sessions ADD CONSTRAINT fk_user_sessions_user FOREIGN KEY (user_id) REFERENCES users(id);
ALTER TABLE audit_logs ADD CONSTRAINT fk_audit_logs_actor_user FOREIGN KEY (actor_user_id) REFERENCES users(id);
ALTER TABLE media_accounts
    ADD CONSTRAINT fk_media_accounts_user FOREIGN KEY (user_id) REFERENCES users(id),
    ADD CONSTRAINT fk_media_accounts_team FOREIGN KEY (team_id) REFERENCES operation_teams(id);
ALTER TABLE media_account_tags ADD CONSTRAINT fk_media_account_tags_user FOREIGN KEY (user_id) REFERENCES users(id);
ALTER TABLE browser_profiles
    ADD CONSTRAINT fk_browser_profiles_user FOREIGN KEY (user_id) REFERENCES users(id),
    ADD CONSTRAINT fk_browser_profiles_team FOREIGN KEY (team_id) REFERENCES operation_teams(id);
ALTER TABLE profile_sync_scans ADD CONSTRAINT fk_profile_sync_scans_user FOREIGN KEY (user_id) REFERENCES users(id);
ALTER TABLE profile_sync_scans ADD CONSTRAINT fk_profile_sync_scans_team FOREIGN KEY (team_id) REFERENCES operation_teams(id);
ALTER TABLE local_agent_binding_tickets ADD CONSTRAINT fk_local_agent_binding_ticket_user FOREIGN KEY (user_id) REFERENCES users(id);
ALTER TABLE local_agent_nodes ADD CONSTRAINT fk_local_agent_node_user FOREIGN KEY (user_id) REFERENCES users(id);
ALTER TABLE browser_profile_runtime_presence ADD CONSTRAINT fk_browser_profile_runtime_presence_user FOREIGN KEY (user_id) REFERENCES users(id);
ALTER TABLE sensitive_browser_tasks ADD CONSTRAINT fk_sensitive_browser_task_user FOREIGN KEY (user_id) REFERENCES users(id);
ALTER TABLE sensitive_profile_permits ADD CONSTRAINT fk_sensitive_profile_permit_user FOREIGN KEY (user_id) REFERENCES users(id);
