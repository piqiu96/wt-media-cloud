-- One row serializes claims across Compose Worker processes and holds the
-- deployment's configured global concurrency. Workers refuse a mismatch.
CREATE TABLE compose_worker_capacity (
    id TINYINT UNSIGNED NOT NULL PRIMARY KEY,
    max_running INT UNSIGNED NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    CONSTRAINT chk_compose_worker_capacity_id CHECK (id = 1),
    CONSTRAINT chk_compose_worker_capacity_positive CHECK (max_running > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO compose_worker_capacity (id, max_running, updated_at) VALUES (1, 2, UTC_TIMESTAMP(6));
