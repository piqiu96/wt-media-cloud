ALTER TABLE compose_tasks
 ADD COLUMN random_seed BIGINT NOT NULL DEFAULT 0,
 ADD COLUMN technical_config_json JSON NULL,
 ADD COLUMN execution_manifest_json JSON NULL;
