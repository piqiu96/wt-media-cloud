-- CHG-20261013-083 (M4-C5): verified compose outputs. A row exists only after
-- the worker uploaded to the temporary key, verified size and sha256, and
-- committed inside the same transaction that marks the task success (spec
-- §6.3). Source references are denormalized from the task so the asset stays
-- readable without joining the task lifecycle. The publish projection starts
-- at pending_publish; published facts belong to M6 publications.
CREATE TABLE composite_outputs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    team_id BIGINT UNSIGNED NOT NULL,
    game_id VARCHAR(32) NOT NULL,
    material_id BIGINT UNSIGNED NOT NULL,
    material_usage_id BIGINT UNSIGNED NOT NULL,
    compose_task_id BIGINT UNSIGNED NOT NULL,
    compose_strategy_id BIGINT UNSIGNED NOT NULL,
    template_code VARCHAR(64) NOT NULL,
    template_version INT UNSIGNED NOT NULL,
    object_key VARCHAR(512) NOT NULL,
    file_size_bytes BIGINT UNSIGNED NOT NULL,
    sha256 CHAR(64) NOT NULL,
    media_json JSON NOT NULL,
    publish_status VARCHAR(16) NOT NULL DEFAULT 'pending_publish',
    created_by BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_composite_outputs_task (compose_task_id),
    UNIQUE KEY uq_composite_outputs_object_key (object_key),
    KEY idx_composite_outputs_scope (team_id, game_id),
    KEY idx_composite_outputs_material (material_id),
    CONSTRAINT fk_composite_outputs_task FOREIGN KEY (compose_task_id) REFERENCES compose_tasks(id),
    CONSTRAINT fk_composite_outputs_team FOREIGN KEY (team_id) REFERENCES operation_teams(id),
    CONSTRAINT fk_composite_outputs_game FOREIGN KEY (game_id) REFERENCES operation_games(id),
    CONSTRAINT fk_composite_outputs_material FOREIGN KEY (material_id) REFERENCES materials(id),
    CONSTRAINT fk_composite_outputs_usage FOREIGN KEY (material_usage_id) REFERENCES material_usages(id),
    CONSTRAINT fk_composite_outputs_strategy FOREIGN KEY (compose_strategy_id) REFERENCES compose_strategies(id),
    CONSTRAINT fk_composite_outputs_created_by FOREIGN KEY (created_by) REFERENCES users(id),
    CONSTRAINT chk_composite_outputs_publish CHECK (publish_status IN ('pending_publish', 'published', 'archived'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Cancelling a running task is a request the worker confirms after the FFmpeg
-- process is gone and temp files are disposed (spec §5.4). One flag on the
-- task row carries it; no second table.
ALTER TABLE compose_tasks
    ADD COLUMN cancel_requested TINYINT(1) NOT NULL DEFAULT 0 AFTER status;
