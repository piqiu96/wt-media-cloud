-- 20260805_018_media_accounts_business_status.sql
-- M2-B 账号收口 B3-2：business_status 对齐 milestone 枚举
--   draft=待识别（创建后未完成真实识别）
--   abnormal=异常（检查发现问题需处理）
-- 已有数据无需回填：历史账号保持原状态（enabled/disabled/retired 仍有效）。

ALTER TABLE media_accounts
    DROP CHECK chk_media_accounts_business_status,
    ADD CONSTRAINT chk_media_accounts_business_status CHECK (business_status IN ('draft', 'enabled', 'disabled', 'abnormal', 'retired'));
