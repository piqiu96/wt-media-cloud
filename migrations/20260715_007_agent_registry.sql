CREATE TABLE IF NOT EXISTS agent_nodes (
    agent_id VARCHAR(64) NOT NULL,
    mode VARCHAR(16) NOT NULL,
    version VARCHAR(32) NOT NULL,
    contract_major_version VARCHAR(8) NOT NULL,
    contract_revision VARCHAR(32) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'online',
    registered_at DATETIME(6) NOT NULL,
    last_heartbeat_at DATETIME(6) NOT NULL,
    capabilities JSON DEFAULT NULL,
    PRIMARY KEY (agent_id),
    KEY idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
