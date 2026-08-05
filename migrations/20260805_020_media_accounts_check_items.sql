-- 20260805_020_media_accounts_check_items.sql
-- M2-B 账号收口 Task4：账号检查 8 项逐项明细结果（最后检查时点快照）
-- 每项：key/label/status(pass|fail|skip|na)/message

ALTER TABLE media_accounts
    ADD COLUMN check_items JSON NULL AFTER last_checked_at;
