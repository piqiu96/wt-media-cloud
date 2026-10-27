-- CHG-20261015-098 (round 3): platform identifiers are stable English codes while
-- display text lives in the client. Baidu's code was previously "baijiahao";
-- rename it to "baidu" so the code matches the platform, not one of its products.
-- The CHECK constraint has to be dropped before the value is rewritten and added
-- back afterwards, otherwise the UPDATE violates it.
ALTER TABLE media_accounts
    DROP CHECK chk_media_accounts_platform;

UPDATE media_accounts
SET platform = 'baidu'
WHERE platform = 'baijiahao';

ALTER TABLE media_accounts
    ADD CONSTRAINT chk_media_accounts_platform CHECK (platform IN ('douyin', 'bilibili', 'baidu'));
