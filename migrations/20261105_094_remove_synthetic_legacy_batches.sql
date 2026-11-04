-- 087 assigned a synthetic batch to each pre-existing task. A task's real
-- originating command is unknown. Undo only the exact, still-linked artifact.
-- This temporary candidate set also prevents an unconfirmed orphan or a real
-- batch that merely resembles a legacy key from entering the delete step.
CREATE TEMPORARY TABLE chg118_legacy_batch_candidates (
 batch_id BIGINT UNSIGNED NOT NULL PRIMARY KEY,
 task_id BIGINT UNSIGNED NOT NULL UNIQUE
) ENGINE=InnoDB;

INSERT INTO chg118_legacy_batch_candidates(batch_id,task_id)
SELECT b.id,t.id
FROM compose_tasks t
JOIN compose_task_batches b ON b.id=t.batch_id
LEFT JOIN compose_tasks other_task ON other_task.batch_id=b.id AND other_task.id<>t.id
LEFT JOIN compose_schedule_runs r ON r.batch_id=b.id
WHERE BINARY b.request_key=BINARY CONCAT('legacy-task-',t.id)
 AND BINARY b.request_hash=BINARY SHA2(CONCAT('legacy-task-',t.id),256)
 AND BINARY b.name=BINARY CONCAT('历史任务 ',t.id)
 AND b.config_snapshot_json=JSON_OBJECT('legacy_task_id',t.id)
 AND b.team_id=t.team_id AND b.category_id=t.category_id
 AND (b.operator_id<=>t.operator_id)
 AND b.compose_strategy_id=t.compose_strategy_id
 AND BINARY b.trigger_type=BINARY t.trigger_type
 AND b.created_by=t.created_by
 AND b.created_at=CAST(t.created_at AS DATETIME(3))
 AND other_task.id IS NULL AND r.run_key IS NULL;

UPDATE compose_tasks t
JOIN chg118_legacy_batch_candidates c ON c.task_id=t.id AND c.batch_id=t.batch_id
SET t.batch_id=NULL;

-- Never delete a batch if any task or scheduling fact still references it.
-- Task, attempt, output and schedule-run facts are otherwise left untouched.
DELETE b FROM compose_task_batches b
JOIN chg118_legacy_batch_candidates c ON c.batch_id=b.id
LEFT JOIN compose_tasks t ON t.batch_id=b.id
LEFT JOIN compose_schedule_runs r ON r.batch_id=b.id
WHERE t.id IS NULL AND r.run_key IS NULL;

DROP TEMPORARY TABLE chg118_legacy_batch_candidates;
