ALTER TABLE compose_tasks
    ADD COLUMN schedule_slot DATETIME(6) NULL AFTER trigger_type,
    ADD UNIQUE KEY uq_compose_tasks_schedule_slot (compose_strategy_id, schedule_slot, material_id),
    ADD KEY idx_compose_tasks_strategy_created (compose_strategy_id, created_at);

CREATE TABLE compose_schedule_runs (
    run_key VARCHAR(64) NOT NULL,
    compose_strategy_id BIGINT UNSIGNED NOT NULL,
    trigger_type VARCHAR(16) NOT NULL,
    schedule_slot DATETIME(6) NOT NULL,
    candidate_count INT UNSIGNED NOT NULL DEFAULT 0,
    created_count INT UNSIGNED NOT NULL DEFAULT 0,
    skipped_json JSON NOT NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (run_key),
    KEY idx_compose_schedule_runs_strategy_slot (compose_strategy_id, schedule_slot),
    CONSTRAINT fk_compose_schedule_runs_strategy FOREIGN KEY (compose_strategy_id) REFERENCES compose_strategies(id),
    CONSTRAINT chk_compose_schedule_runs_trigger CHECK (trigger_type IN ('scheduled', 'manual'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
