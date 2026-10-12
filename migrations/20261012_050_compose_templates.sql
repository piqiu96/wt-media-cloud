-- Establish versioned template storage and remove the superseded strategy version model.
-- Publish template records only after their executor code is deployed.

CREATE TABLE compose_templates (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    code VARCHAR(64) NOT NULL,
    name VARCHAR(120) NOT NULL,
    version INT UNSIGNED NOT NULL,
    schema_json JSON NOT NULL,
    executor VARCHAR(64) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_compose_templates_code_version (code, version),
    CONSTRAINT chk_compose_templates_version CHECK (version > 0),
    CONSTRAINT chk_compose_templates_status CHECK (status IN ('active', 'inactive'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

ALTER TABLE compose_strategies
    ADD COLUMN compose_template_id BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER game_id,
    ADD COLUMN compose_template_version INT UNSIGNED NOT NULL DEFAULT 1 AFTER compose_template_id,
    DROP COLUMN template_key,
    DROP COLUMN template_version,
    DROP COLUMN current_version,
    DROP CHECK chk_compose_strategies_version,
    ADD CONSTRAINT fk_compose_strategies_template FOREIGN KEY (compose_template_id) REFERENCES compose_templates(id);

DROP TABLE compose_strategy_versions;
