-- 20260806_021_media_accounts_business_status_simplify.sql
-- 社媒账号列表重构：业务状态简化为 启用/停用（enabled/disabled）
-- 真实状态（待上号/异常/正常等）由 账号状态（login_status + identification_status）承载。
-- 历史 draft(待识别)/abnormal(异常) → enabled（业务状态不承载真实健康度）。

UPDATE media_accounts SET business_status = 'enabled' WHERE business_status IN ('draft', 'abnormal');

ALTER TABLE media_accounts
    DROP CHECK chk_media_accounts_business_status,
    ADD CONSTRAINT chk_media_accounts_business_status CHECK (business_status IN ('enabled', 'disabled'));
