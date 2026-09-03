-- 20260814_024_media_account_tags_unique_account_scope.sql
-- 修复：media_account_tags 唯一键被迁移 022 错误折叠为 (user_id, tag_name)，缺少 media_account_id，
-- 导致同一用户跨账号复用同一标签名被 INSERT IGNORE 静默丢弃。
-- 恢复为 (user_id, media_account_id, tag_name)，使标签按「用户+账号+标签名」唯一。
ALTER TABLE media_account_tags
    DROP INDEX uq_media_account_tags_owner_account_name,
    ADD UNIQUE KEY uq_media_account_tags_owner_account_name (user_id, media_account_id, tag_name);
