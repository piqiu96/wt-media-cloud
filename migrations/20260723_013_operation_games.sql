CREATE TABLE operation_games (
    id VARCHAR(32) NOT NULL,
    name VARCHAR(128) NOT NULL,
    status VARCHAR(16) NOT NULL,
    remark VARCHAR(255) NOT NULL DEFAULT '',
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_operation_games_name (name),
    CONSTRAINT chk_operation_games_id CHECK (id REGEXP '^[A-Za-z0-9]+$'),
    CONSTRAINT chk_operation_games_status CHECK (status IN ('enabled', 'disabled'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO operation_games (id, name, status, remark, created_at, updated_at)
SELECT DISTINCT scopes.game_id, scopes.game_id, 'enabled', '从已有用户游戏范围迁移', UTC_TIMESTAMP(6), UTC_TIMESTAMP(6)
FROM user_game_scopes scopes
WHERE scopes.game_id <> '';
