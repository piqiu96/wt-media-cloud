-- A recovered successful task must not display an earlier attempt's failure.
-- Attempt rows retain the complete failure and verification history.
UPDATE compose_tasks SET error_code = '', error_message = ''
WHERE status = 'success' AND (COALESCE(error_code,'') <> '' OR COALESCE(error_message,'') <> '');
