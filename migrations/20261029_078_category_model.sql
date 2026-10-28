-- Replace the string-keyed game catalog with a flat, auto-increment category
-- catalog. Existing rows stay in the `game` category type; numeric IDs are the
-- only identity exposed by the new model.

CREATE TABLE categories (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    type VARCHAR(32) NOT NULL,
    name VARCHAR(128) NOT NULL,
    enabled TINYINT(1) NOT NULL DEFAULT 1,
    remark VARCHAR(255) NOT NULL DEFAULT '',
    legacy_id VARCHAR(32) NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_categories_name (name),
    UNIQUE KEY uq_categories_legacy_id (legacy_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO categories (type, name, enabled, remark, legacy_id, created_at, updated_at)
SELECT 'game', name, status = 'enabled', remark, id, created_at, updated_at
FROM operation_games;

CREATE TEMPORARY TABLE category_id_map (
    legacy_id VARCHAR(32) NOT NULL,
    category_id BIGINT UNSIGNED NOT NULL,
    PRIMARY KEY (legacy_id)
);

INSERT INTO category_id_map (legacy_id, category_id)
SELECT legacy_id, id FROM categories WHERE legacy_id IS NOT NULL;

ALTER TABLE compose_strategies ADD COLUMN category_id BIGINT UNSIGNED NULL AFTER team_id;
UPDATE compose_strategies child JOIN category_id_map map ON map.legacy_id = child.game_id SET child.category_id = map.category_id;

ALTER TABLE compose_tasks ADD COLUMN category_id BIGINT UNSIGNED NULL AFTER team_id;
UPDATE compose_tasks child JOIN category_id_map map ON map.legacy_id = child.game_id SET child.category_id = map.category_id;

ALTER TABLE composite_outputs ADD COLUMN category_id BIGINT UNSIGNED NULL AFTER team_id;
UPDATE composite_outputs child JOIN category_id_map map ON map.legacy_id = child.game_id SET child.category_id = map.category_id;

ALTER TABLE discovery_strategies ADD COLUMN category_id BIGINT UNSIGNED NULL AFTER team_id;
UPDATE discovery_strategies child JOIN category_id_map map ON map.legacy_id = child.game_id SET child.category_id = map.category_id;

ALTER TABLE source_contents ADD COLUMN category_id BIGINT UNSIGNED NULL AFTER team_id;
UPDATE source_contents child JOIN category_id_map map ON map.legacy_id = child.game_id SET child.category_id = map.category_id;

ALTER TABLE user_game_scopes ADD COLUMN category_id BIGINT UNSIGNED NULL AFTER user_id;
UPDATE user_game_scopes child JOIN category_id_map map ON map.legacy_id = child.game_id SET child.category_id = map.category_id;

ALTER TABLE media_account_games ADD COLUMN category_id BIGINT UNSIGNED NULL AFTER media_account_id;
UPDATE media_account_games child JOIN category_id_map map ON map.legacy_id = child.game_id SET child.category_id = map.category_id;

-- Guard every required legacy reference before destructive column changes.
CREATE TEMPORARY TABLE category_migration_guard (
    valid TINYINT NOT NULL,
    CONSTRAINT chk_category_migration_guard CHECK (valid = 1)
);
INSERT INTO category_migration_guard (valid)
SELECT IF(
    (SELECT COUNT(*) FROM compose_strategies WHERE game_id IS NOT NULL AND category_id IS NULL)
    + (SELECT COUNT(*) FROM compose_tasks WHERE game_id IS NOT NULL AND category_id IS NULL)
    + (SELECT COUNT(*) FROM composite_outputs WHERE game_id IS NOT NULL AND category_id IS NULL)
    + (SELECT COUNT(*) FROM discovery_strategies WHERE game_id IS NOT NULL AND category_id IS NULL)
    + (SELECT COUNT(*) FROM source_contents WHERE game_id IS NOT NULL AND category_id IS NULL)
    + (SELECT COUNT(*) FROM user_game_scopes WHERE game_id IS NOT NULL AND category_id IS NULL)
    + (SELECT COUNT(*) FROM media_account_games WHERE game_id IS NOT NULL AND category_id IS NULL) = 0,
    1,
    0
);
DROP TEMPORARY TABLE category_migration_guard;

ALTER TABLE compose_strategies
    DROP FOREIGN KEY fk_compose_strategies_game,
    DROP INDEX idx_compose_strategies_scope,
    DROP COLUMN game_id,
    CHANGE COLUMN category_id category_id BIGINT UNSIGNED NOT NULL,
    ADD KEY idx_compose_strategies_scope (team_id, category_id, status),
    ADD CONSTRAINT fk_compose_strategies_category FOREIGN KEY (category_id) REFERENCES categories(id);

ALTER TABLE compose_tasks
    DROP FOREIGN KEY fk_compose_tasks_game,
    DROP INDEX idx_compose_tasks_scope_status,
    DROP COLUMN game_id,
    CHANGE COLUMN category_id category_id BIGINT UNSIGNED NOT NULL,
    ADD KEY idx_compose_tasks_scope_status (team_id, category_id, status),
    ADD CONSTRAINT fk_compose_tasks_category FOREIGN KEY (category_id) REFERENCES categories(id);

ALTER TABLE composite_outputs
    DROP FOREIGN KEY fk_composite_outputs_game,
    DROP INDEX idx_composite_outputs_scope,
    DROP COLUMN game_id,
    CHANGE COLUMN category_id category_id BIGINT UNSIGNED NOT NULL,
    ADD KEY idx_composite_outputs_scope (team_id, category_id),
    ADD CONSTRAINT fk_composite_outputs_category FOREIGN KEY (category_id) REFERENCES categories(id);

ALTER TABLE discovery_strategies
    DROP FOREIGN KEY fk_discovery_strategies_game,
    DROP INDEX idx_discovery_strategies_game,
    DROP COLUMN game_id,
    ADD KEY idx_discovery_strategies_category (category_id),
    ADD CONSTRAINT fk_discovery_strategies_category FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE SET NULL;

ALTER TABLE source_contents
    DROP FOREIGN KEY fk_source_contents_game,
    DROP INDEX idx_source_contents_game,
    DROP COLUMN game_id,
    ADD KEY idx_source_contents_category (category_id),
    ADD CONSTRAINT fk_source_contents_category FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE SET NULL;

ALTER TABLE user_game_scopes
    DROP PRIMARY KEY,
    DROP COLUMN game_id,
    CHANGE COLUMN category_id category_id BIGINT UNSIGNED NOT NULL,
    ADD PRIMARY KEY (user_id, category_id),
    ADD KEY idx_user_category_scopes_category (category_id),
    ADD CONSTRAINT fk_user_category_scopes_category FOREIGN KEY (category_id) REFERENCES categories(id);
RENAME TABLE user_game_scopes TO user_category_scopes;

ALTER TABLE media_account_games
    DROP FOREIGN KEY fk_media_account_games_game,
    DROP INDEX idx_media_account_games_game_account,
    DROP PRIMARY KEY,
    DROP COLUMN game_id,
    CHANGE COLUMN category_id category_id BIGINT UNSIGNED NOT NULL,
    ADD PRIMARY KEY (media_account_id, category_id),
    ADD KEY idx_media_account_categories_category (category_id),
    ADD CONSTRAINT fk_media_account_categories_account FOREIGN KEY (media_account_id) REFERENCES media_accounts(id),
    ADD CONSTRAINT fk_media_account_categories_category FOREIGN KEY (category_id) REFERENCES categories(id);
RENAME TABLE media_account_games TO media_account_categories;

DROP TABLE operation_games;
DROP TEMPORARY TABLE category_id_map;
ALTER TABLE categories DROP INDEX uq_categories_legacy_id, DROP COLUMN legacy_id;
