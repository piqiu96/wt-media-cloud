-- Repair files created by an uploader that wrote the Go zero value.
-- Their bytes and uploaded_at remain unchanged; deleting is never restored.
UPDATE operations_files SET delete_state='ready' WHERE delete_state='';
ALTER TABLE operations_files ADD CONSTRAINT chk_operations_file_delete_state CHECK (delete_state IN ('ready','deleting'));
