CREATE TABLE IF NOT EXISTS tasks (
    task_id VARCHAR(64) NOT NULL,
    task_type VARCHAR(32) NOT NULL,
    status VARCHAR(16) NOT NULL,
    idempotency_key VARCHAR(128) DEFAULT NULL,
    agent_id VARCHAR(64) DEFAULT NULL,
    created_at DATETIME(6) NOT NULL,
    lease_expires_at DATETIME(6) DEFAULT NULL,
    progress INT NOT NULL DEFAULT 0,
    message TEXT DEFAULT NULL,
    updated_at DATETIME(6) DEFAULT NULL,
    error_code VARCHAR(64) DEFAULT NULL,
    PRIMARY KEY (task_id),
    UNIQUE KEY uq_idempotency_key (idempotency_key),
    KEY idx_status (status),
    KEY idx_agent_id (agent_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
