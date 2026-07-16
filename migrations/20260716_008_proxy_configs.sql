-- M2-C5: proxy_configs 代理管理
CREATE TABLE proxy_configs (
    id VARCHAR(64) NOT NULL,
    proxy_protocol VARCHAR(16) NOT NULL DEFAULT 'http',
    host VARCHAR(255) NOT NULL,
    port INT NOT NULL,
    username VARCHAR(128) DEFAULT '',
    password VARCHAR(255) DEFAULT '',
    region VARCHAR(128) DEFAULT '',
    supplier VARCHAR(128) DEFAULT '',
    expires_at DATETIME(6) DEFAULT NULL,
    business_status VARCHAR(16) NOT NULL DEFAULT 'active',
    last_check_at DATETIME(6) DEFAULT NULL,
    last_check_result VARCHAR(64) DEFAULT '',
    observed_exit_ip VARCHAR(64) DEFAULT '',
    remark TEXT,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    INDEX idx_proxy_configs_status (business_status),
    INDEX idx_proxy_configs_supplier (supplier)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE proxy_platform_quotas (
    proxy_id VARCHAR(64) NOT NULL,
    platform VARCHAR(32) NOT NULL,
    max_profiles INT NOT NULL DEFAULT 1,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (proxy_id, platform),
    CONSTRAINT fk_proxy_quotas_proxy FOREIGN KEY (proxy_id) REFERENCES proxy_configs(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
