-- V3 浏览器窗口与代理管理：代理从全局资源池改为按团队共享。
--
-- 用户裁决（2026-10-28）：存量代理不做团队回填，直接整体清理；此后新增代理必须带团队。
-- `browser_profiles.proxy_id -> proxy_configs` 是 ON DELETE SET NULL，DELETE 会同时把
-- 窗口上失效的正式关联置空，窗口本身、账号关系和备注不受影响。
DELETE FROM proxy_configs;

ALTER TABLE proxy_configs
    ADD COLUMN team_id BIGINT UNSIGNED NOT NULL AFTER id,
    ADD COLUMN name VARCHAR(128) NOT NULL DEFAULT '' AFTER team_id,
    ADD COLUMN tags_json TEXT NULL AFTER supplier,
    ADD INDEX idx_proxy_configs_team (team_id, business_status),
    ADD CONSTRAINT fk_proxy_configs_team FOREIGN KEY (team_id) REFERENCES operation_teams (id);
