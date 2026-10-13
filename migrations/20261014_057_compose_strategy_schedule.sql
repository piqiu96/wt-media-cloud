ALTER TABLE compose_strategies
    ADD COLUMN schedule_json JSON NULL AFTER config_json,
    ADD COLUMN next_run_at DATETIME(6) NULL AFTER schedule_json,
    ADD KEY idx_compose_strategies_next_run (status, next_run_at);
