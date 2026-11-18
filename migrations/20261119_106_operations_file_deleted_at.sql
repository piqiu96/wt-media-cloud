-- Preserve in-progress deletions, including failed physical deletion/settlement.
ALTER TABLE operations_files ADD COLUMN deleted_at DATETIME(6) NULL;
UPDATE operations_files SET deleted_at=updated_at WHERE delete_state='deleting';
ALTER TABLE operations_files DROP CHECK chk_operations_file_delete_state;
ALTER TABLE operations_files DROP COLUMN delete_state;
