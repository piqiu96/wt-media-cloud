-- 20260806_022_media_accounts_id_auto_increment.sql
-- media_accounts.id: VARCHAR(64) 字符串 → BIGINT UNSIGNED AUTO_INCREMENT（遵循"主键一律自增"）
-- 旧字符串 id 暂存 legacy_id（迁移 023 清除），历史关联通过 JOIN 重指。
-- 引用 media_accounts.id 的列同步迁移：media_account_tags.media_account_id、duplicate_of_account_id、audit_logs.target_id。

-- 1. 先删引用 media_accounts.id 的外键
ALTER TABLE media_account_tags DROP FOREIGN KEY fk_media_account_tags_account;
ALTER TABLE media_accounts DROP FOREIGN KEY fk_media_accounts_duplicate;

-- 2. media_accounts.id: 字符串 PK → legacy_id + 新自增 BIGINT PK
ALTER TABLE media_accounts
    DROP PRIMARY KEY,
    CHANGE COLUMN id legacy_id VARCHAR(64) NULL,
    ADD COLUMN id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT FIRST,
    ADD UNIQUE KEY uq_media_accounts_legacy_id (legacy_id),
    ADD PRIMARY KEY (id);

-- 3. media_account_tags.media_account_id → BIGINT 重指
ALTER TABLE media_account_tags
    ADD COLUMN media_account_id_new BIGINT UNSIGNED NULL AFTER media_account_id,
    ADD KEY idx_media_account_tags_maid_new (media_account_id_new);
UPDATE media_account_tags t JOIN media_accounts ma ON ma.legacy_id = t.media_account_id
    SET t.media_account_id_new = ma.id;
ALTER TABLE media_account_tags
    DROP COLUMN media_account_id,
    CHANGE COLUMN media_account_id_new media_account_id BIGINT UNSIGNED NOT NULL,
    ADD CONSTRAINT fk_media_account_tags_account FOREIGN KEY (media_account_id) REFERENCES media_accounts(id);

-- 4. duplicate_of_account_id 自引用 → BIGINT 重指
ALTER TABLE media_accounts
    ADD COLUMN duplicate_of_account_id_new BIGINT UNSIGNED NULL AFTER duplicate_of_account_id;
UPDATE media_accounts a JOIN media_accounts b ON b.legacy_id = a.duplicate_of_account_id
    SET a.duplicate_of_account_id_new = b.id;
ALTER TABLE media_accounts
    DROP COLUMN duplicate_of_account_id,
    CHANGE COLUMN duplicate_of_account_id_new duplicate_of_account_id BIGINT UNSIGNED NULL,
    ADD CONSTRAINT fk_media_accounts_duplicate FOREIGN KEY (duplicate_of_account_id) REFERENCES media_accounts(id);

-- 5. audit_logs.target_id（松引用，无 FK）：按 legacy 映射重指（历史审计）
UPDATE audit_logs a JOIN media_accounts ma ON ma.legacy_id = a.target_id
    SET a.target_id = ma.id;
