CREATE TABLE compose_strategies (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    team_id BIGINT UNSIGNED NOT NULL,
    game_id VARCHAR(32) NOT NULL,
    name VARCHAR(120) NOT NULL,
    content_type VARCHAR(64) NOT NULL,
    template_key VARCHAR(64) NOT NULL,
    template_version VARCHAR(32) NOT NULL,
    parameters_json JSON NOT NULL,
    allowed_overrides_json JSON NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    current_version INT UNSIGNED NOT NULL DEFAULT 1,
    created_by BIGINT UNSIGNED NOT NULL,
    updated_by BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    KEY idx_compose_strategies_scope (team_id, game_id, status),
    CONSTRAINT fk_compose_strategies_team FOREIGN KEY (team_id) REFERENCES operation_teams(id),
    CONSTRAINT fk_compose_strategies_game FOREIGN KEY (game_id) REFERENCES operation_games(id),
    CONSTRAINT fk_compose_strategies_created_by FOREIGN KEY (created_by) REFERENCES users(id),
    CONSTRAINT fk_compose_strategies_updated_by FOREIGN KEY (updated_by) REFERENCES users(id),
    CONSTRAINT chk_compose_strategies_status CHECK (status IN ('active', 'inactive')),
    CONSTRAINT chk_compose_strategies_version CHECK (current_version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE compose_strategy_versions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    strategy_id BIGINT UNSIGNED NOT NULL,
    version INT UNSIGNED NOT NULL,
    snapshot_json JSON NOT NULL,
    created_by BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_compose_strategy_version (strategy_id, version),
    CONSTRAINT fk_compose_strategy_versions_strategy FOREIGN KEY (strategy_id) REFERENCES compose_strategies(id),
    CONSTRAINT fk_compose_strategy_versions_created_by FOREIGN KEY (created_by) REFERENCES users(id),
    CONSTRAINT chk_compose_strategy_versions_version CHECK (version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
