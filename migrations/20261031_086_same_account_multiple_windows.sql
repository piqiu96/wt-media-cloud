-- Platform identity may be used in several windows. The window/platform relation stays unique.
ALTER TABLE media_accounts
    DROP FOREIGN KEY fk_media_accounts_duplicate,
    DROP INDEX uq_media_accounts_user_platform_identity,
    ADD INDEX idx_media_accounts_user_platform_identity (user_id, platform, platform_account_id);

-- Historical duplicate verdicts did not save a verified identity to this window.
-- Preserve observations/credentials, and require a new read-only check instead of inventing normal login.
UPDATE media_accounts SET identification_status='pending_identification', platform_login_status='unknown',
    last_check_status='never', last_check_message='请检查当前窗口，确认账号信息', check_operation_id=NULL,
    check_profile_id=NULL, check_started_at=NULL
WHERE identification_status='duplicate';
ALTER TABLE media_accounts DROP CHECK chk_media_accounts_identification,
    DROP COLUMN duplicate_of_account_id,
    ADD CONSTRAINT chk_media_accounts_identification CHECK (identification_status IN ('pending_identification', 'identified'));

-- Recover the original array order and inputs. Never replay Cookie writes for old duplicate rows.
CREATE TEMPORARY TABLE chg114_import_items AS
SELECT b.id, j.ordinality,
    CASE WHEN JSON_UNQUOTE(JSON_EXTRACT(j.item, '$.existing_account_id')) IS NOT NULL THEN
        JSON_REMOVE(JSON_SET(j.item,
            '$.same_identity_account_id', JSON_EXTRACT(j.item, '$.existing_account_id'),
            '$.status', IF(JSON_UNQUOTE(JSON_EXTRACT(j.item, '$.status'))='failed', 'uncertain', JSON_UNQUOTE(JSON_EXTRACT(j.item, '$.status'))),
            '$.previous_status', IF(JSON_UNQUOTE(JSON_EXTRACT(j.item, '$.previous_status'))='failed', 'uncertain', COALESCE(JSON_UNQUOTE(JSON_EXTRACT(j.item, '$.previous_status')), 'pending')),
            '$.message', '请核实当前窗口，完成本行接入'), '$.existing_account_id')
    ELSE j.item END AS item
FROM media_account_import_batches b
JOIN JSON_TABLE(b.content_json, '$.items[*]' COLUMNS(ordinality FOR ORDINALITY, item JSON PATH '$')) j
WHERE JSON_CONTAINS_PATH(b.content_json, 'one', '$.items[*].existing_account_id');
UPDATE media_account_import_batches b
JOIN (
    SELECT id, items FROM (
        SELECT id,
            JSON_ARRAYAGG(CAST(item AS JSON)) OVER (PARTITION BY id ORDER BY ordinality ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING) AS items,
            ROW_NUMBER() OVER (PARTITION BY id ORDER BY ordinality) AS first_item
        FROM chg114_import_items
    ) ordered_items WHERE first_item=1
) rewritten ON rewritten.id=b.id
SET b.content_json=JSON_SET(b.content_json, '$.items', rewritten.items), b.revision=b.revision+1;
DROP TEMPORARY TABLE chg114_import_items;
