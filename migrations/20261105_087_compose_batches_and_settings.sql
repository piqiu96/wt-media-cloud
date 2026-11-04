-- Content production: real command batches; settings remain JSON, not another table.
ALTER TABLE compose_strategies ADD COLUMN output_settings_json JSON NULL;
CREATE TABLE compose_task_batches (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
 name VARCHAR(180) NOT NULL,
 team_id BIGINT UNSIGNED NOT NULL,
 category_id BIGINT UNSIGNED NOT NULL,
 operator_id BIGINT UNSIGNED NULL,
 compose_strategy_id BIGINT UNSIGNED NOT NULL,
 trigger_type VARCHAR(16) NOT NULL,
 created_by BIGINT UNSIGNED NOT NULL,
 request_key VARCHAR(64) NOT NULL,
 request_hash CHAR(64) NOT NULL,
 config_snapshot_json JSON NOT NULL,
 created_at DATETIME(3) NOT NULL,
 UNIQUE KEY uq_compose_batch_request(request_key),
 KEY idx_compose_batch_scope(team_id,operator_id,created_at),
 CONSTRAINT fk_compose_batch_strategy FOREIGN KEY(compose_strategy_id) REFERENCES compose_strategies(id),
 CONSTRAINT chk_compose_batch_trigger CHECK(trigger_type IN ('manual','immediate','scheduled'))
);
ALTER TABLE compose_tasks
 ADD COLUMN batch_id BIGINT UNSIGNED NULL,
 ADD COLUMN setting_key VARCHAR(64) NOT NULL DEFAULT 'default',
 ADD COLUMN setting_name VARCHAR(120) NOT NULL DEFAULT '默认',
 ADD COLUMN platforms_json JSON NULL,
 ADD COLUMN copy_index INT NOT NULL DEFAULT 1,
 DROP INDEX uq_compose_tasks_schedule_slot,
 ADD UNIQUE KEY uq_compose_batch_material_setting_copy(batch_id,material_id,setting_key,copy_index),
 ADD KEY idx_compose_task_batch(batch_id,id),
 ADD CONSTRAINT fk_compose_task_batch FOREIGN KEY(batch_id) REFERENCES compose_task_batches(id);
ALTER TABLE compose_schedule_runs ADD COLUMN batch_id BIGINT UNSIGNED NULL,
 ADD CONSTRAINT fk_compose_run_batch FOREIGN KEY(batch_id) REFERENCES compose_task_batches(id);
-- Historical task identity is preserved. Its originating command is unknown: one explicit legacy batch per task.
INSERT INTO compose_task_batches(name,team_id,category_id,operator_id,compose_strategy_id,trigger_type,created_by,request_key,request_hash,config_snapshot_json,created_at)
 SELECT CONCAT('历史任务 ',id),team_id,category_id,operator_id,compose_strategy_id,trigger_type,created_by,CONCAT('legacy-task-',id),SHA2(CONCAT('legacy-task-',id),256),JSON_OBJECT('legacy_task_id',id),created_at FROM compose_tasks;
UPDATE compose_tasks t JOIN compose_task_batches b ON b.request_key=CONCAT('legacy-task-',t.id) SET t.batch_id=b.id;

ALTER TABLE compose_tasks DROP CHECK chk_compose_tasks_trigger, ADD CONSTRAINT chk_compose_tasks_trigger CHECK(trigger_type IN ('manual','immediate','scheduled'));
ALTER TABLE compose_schedule_runs DROP CHECK chk_compose_schedule_runs_trigger, ADD CONSTRAINT chk_compose_schedule_runs_trigger CHECK(trigger_type IN ('manual','immediate','scheduled'));
