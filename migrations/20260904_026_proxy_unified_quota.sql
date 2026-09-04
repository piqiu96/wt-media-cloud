-- M2-C Task 2: one proxy has one cross-platform Profile capacity.
ALTER TABLE proxy_configs
    ADD COLUMN max_profile_count INT NOT NULL DEFAULT 3 AFTER business_status;

UPDATE proxy_configs proxies
LEFT JOIN (
    SELECT proxy_id, MAX(max_profiles) AS max_profile_count
    FROM proxy_platform_quotas
    GROUP BY proxy_id
) legacy ON legacy.proxy_id = proxies.id
SET proxies.max_profile_count = COALESCE(legacy.max_profile_count, 3);

ALTER TABLE browser_profiles
    ADD COLUMN proxy_id VARCHAR(64) NULL AFTER proxy_port,
    ADD INDEX idx_browser_profiles_proxy_id (proxy_id),
    ADD CONSTRAINT fk_browser_profiles_proxy FOREIGN KEY (proxy_id) REFERENCES proxy_configs(id) ON DELETE SET NULL;

DROP TABLE proxy_platform_quotas;
