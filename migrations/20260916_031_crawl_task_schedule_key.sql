ALTER TABLE crawl_tasks
    ADD COLUMN schedule_key VARCHAR(128) NULL AFTER strategy_id,
    ADD UNIQUE KEY uq_crawl_tasks_strategy_schedule (strategy_id, schedule_key);
