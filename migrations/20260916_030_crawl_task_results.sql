ALTER TABLE crawl_tasks
    ADD COLUMN result_json JSON NULL AFTER stats_json;
