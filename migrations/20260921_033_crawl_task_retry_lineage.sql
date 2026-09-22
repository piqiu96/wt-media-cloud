ALTER TABLE crawl_tasks
    ADD COLUMN parent_task_id BIGINT UNSIGNED NULL AFTER strategy_id,
    ADD KEY idx_crawl_tasks_parent (parent_task_id),
    ADD CONSTRAINT fk_crawl_tasks_parent FOREIGN KEY (parent_task_id) REFERENCES crawl_tasks(id);

ALTER TABLE crawl_tasks
    DROP CHECK chk_crawl_tasks_status;

ALTER TABLE crawl_tasks
    ADD CONSTRAINT chk_crawl_tasks_status CHECK (status IN ('pending', 'running', 'success', 'partial_success', 'failed'));
