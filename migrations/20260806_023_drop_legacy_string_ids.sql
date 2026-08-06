-- 20260806_023_drop_legacy_string_ids.sql
-- 主键已全部迁移自增后，清理遗留的旧字符串 ID 列。
-- media_accounts.legacy_id（022 迁移暂存）、users.legacy_id（012 迁移保留）。
-- browser_profiles.legacy_id 已在 017 迁移内清除。

ALTER TABLE media_accounts DROP INDEX uq_media_accounts_legacy_id, DROP COLUMN legacy_id;
ALTER TABLE users DROP INDEX uq_users_legacy_id, DROP COLUMN legacy_id;
