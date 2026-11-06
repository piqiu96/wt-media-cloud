CREATE TABLE compose_output_allocations (
    id BIGINT NOT NULL AUTO_INCREMENT,
    task_id BIGINT NOT NULL,
    attempt INT NOT NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_compose_output_allocation_attempt (task_id, attempt)
);

INSERT INTO compose_output_allocations (id, task_id, attempt, created_at)
SELECT id, compose_task_id, 0, created_at FROM composite_outputs;
