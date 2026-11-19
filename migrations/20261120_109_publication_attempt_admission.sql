-- Publish owns admission. Ordinary resource reads do not lock owner rows.
-- Run with Cloud writers stopped. Existing duplicate Profiles or >3 active
-- attempts per user fail the constraints below; verify data, never free it silently.
ALTER TABLE publication_attempt ADD COLUMN admission_slot TINYINT UNSIGNED NOT NULL DEFAULT 1;

UPDATE publication_attempt a
JOIN (
 SELECT id, ROW_NUMBER() OVER (PARTITION BY executor_user_id ORDER BY id) AS slot
 FROM publication_attempt
 WHERE status IN ('running','awaiting_human') OR (status='interrupted' AND resolved_at IS NULL)
) occupied ON occupied.id=a.id
SET a.admission_slot=occupied.slot;

ALTER TABLE publication_attempt
 ADD CONSTRAINT chk_attempt_admission_slot CHECK (admission_slot BETWEEN 1 AND 3),
 ADD COLUMN active_profile_id BIGINT UNSIGNED GENERATED ALWAYS AS
   (CASE WHEN status IN ('running','awaiting_human') OR (status='interrupted' AND resolved_at IS NULL) THEN browser_profile_id ELSE NULL END) STORED,
 ADD COLUMN active_admission_slot TINYINT UNSIGNED GENERATED ALWAYS AS
   (CASE WHEN status IN ('running','awaiting_human') OR (status='interrupted' AND resolved_at IS NULL) THEN admission_slot ELSE NULL END) STORED,
 ADD UNIQUE KEY uq_attempt_active_profile (active_profile_id),
 ADD UNIQUE KEY uq_attempt_user_slot (executor_user_id,active_admission_slot);
-- Rollback only with writers stopped: drop both indexes, both generated columns,
-- chk_attempt_admission_slot, then admission_slot; restore old admission protection.
