-- CHG-20261028-099 / ADR-0029: preserve existing proxy bindings while
-- replacing the public and primary proxy id with an auto-increment integer.
ALTER TABLE browser_profiles
    DROP FOREIGN KEY fk_browser_profiles_proxy,
    DROP FOREIGN KEY fk_browser_profiles_reserved_proxy;

ALTER TABLE proxies
    ADD COLUMN numeric_id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT UNIQUE;

ALTER TABLE browser_profiles
    ADD COLUMN numeric_proxy_id BIGINT UNSIGNED NULL AFTER proxy_id,
    ADD COLUMN numeric_reserved_proxy_id BIGINT UNSIGNED NULL AFTER reserved_proxy_id;

UPDATE browser_profiles bp
JOIN proxies p ON p.id = bp.proxy_id
SET bp.numeric_proxy_id = p.numeric_id;

UPDATE browser_profiles bp
JOIN proxies p ON p.id = bp.reserved_proxy_id
SET bp.numeric_reserved_proxy_id = p.numeric_id;

UPDATE browser_profiles bp
LEFT JOIN proxies p ON p.id = JSON_UNQUOTE(JSON_EXTRACT(bp.proxy_operation, '$.proxy_id'))
SET bp.proxy_operation = JSON_SET(bp.proxy_operation, '$.proxy_id', p.numeric_id)
WHERE bp.proxy_operation IS NOT NULL;

ALTER TABLE browser_profiles
    DROP INDEX idx_browser_profiles_proxy,
    DROP INDEX idx_browser_profiles_reserved_proxy,
    DROP COLUMN proxy_id,
    DROP COLUMN reserved_proxy_id,
    CHANGE COLUMN numeric_proxy_id proxy_id BIGINT UNSIGNED NULL,
    CHANGE COLUMN numeric_reserved_proxy_id reserved_proxy_id BIGINT UNSIGNED NULL,
    ADD KEY idx_browser_profiles_proxy (proxy_id),
    ADD KEY idx_browser_profiles_reserved_proxy (reserved_proxy_id);

ALTER TABLE proxies
    DROP PRIMARY KEY,
    CHANGE COLUMN id legacy_id VARCHAR(64) NULL,
    CHANGE COLUMN numeric_id id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    ADD PRIMARY KEY (id),
    ADD UNIQUE KEY uq_proxies_legacy_id (legacy_id),
    DROP INDEX numeric_id;

ALTER TABLE browser_profiles
    ADD CONSTRAINT fk_browser_profiles_proxy FOREIGN KEY (proxy_id) REFERENCES proxies(id) ON DELETE RESTRICT,
    ADD CONSTRAINT fk_browser_profiles_reserved_proxy FOREIGN KEY (reserved_proxy_id) REFERENCES proxies(id) ON DELETE RESTRICT;
