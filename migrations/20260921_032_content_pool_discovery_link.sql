ALTER TABLE source_contents
    ADD COLUMN strategy_id BIGINT UNSIGNED NULL AFTER source_type,
    ADD COLUMN crawl_task_id BIGINT UNSIGNED NULL AFTER strategy_id,
    ADD COLUMN like_count BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER crawl_task_id,
    ADD COLUMN favorite_count BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER like_count,
    ADD COLUMN audit_note VARCHAR(500) NULL AFTER ignored_reason,
    ADD COLUMN failure_reason VARCHAR(500) NULL AFTER audit_note,
    ADD COLUMN material_id BIGINT UNSIGNED NULL AFTER failure_reason,
    ADD KEY idx_source_contents_strategy (strategy_id),
    ADD KEY idx_source_contents_crawl_task (crawl_task_id),
    ADD CONSTRAINT fk_source_contents_strategy FOREIGN KEY (strategy_id) REFERENCES discovery_strategies(id),
    ADD CONSTRAINT fk_source_contents_crawl_task FOREIGN KEY (crawl_task_id) REFERENCES crawl_tasks(id);
