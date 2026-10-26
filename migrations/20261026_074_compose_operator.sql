-- CHG-20261026-103 (M4-I): one production operator per strategy, frozen on each
-- task at creation and inherited by the verified output. Existing rows keep
-- NULL: the repository has no verifiable source for a historical operator, and
-- created_by must not be promoted to one. API models expose the unknown as
-- null; pages show 未记录.
ALTER TABLE compose_strategies
    ADD COLUMN operator_id BIGINT UNSIGNED NULL AFTER game_id,
    ADD KEY idx_compose_strategies_operator (team_id, operator_id, status),
    ADD CONSTRAINT fk_compose_strategies_operator FOREIGN KEY (operator_id) REFERENCES users(id);

ALTER TABLE compose_tasks
    ADD COLUMN operator_id BIGINT UNSIGNED NULL AFTER game_id,
    ADD KEY idx_compose_tasks_operator (team_id, operator_id),
    ADD CONSTRAINT fk_compose_tasks_operator FOREIGN KEY (operator_id) REFERENCES users(id);

ALTER TABLE composite_outputs
    ADD COLUMN operator_id BIGINT UNSIGNED NULL AFTER game_id,
    ADD KEY idx_composite_outputs_operator (team_id, operator_id),
    ADD CONSTRAINT fk_composite_outputs_operator FOREIGN KEY (operator_id) REFERENCES users(id);
