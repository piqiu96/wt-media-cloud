-- The verification note stores the complete typed evidence, not only its basis.
-- Keep the existing input limits while retaining JSON keys, reason and Unicode.
ALTER TABLE publication_attempt MODIFY COLUMN resolution_note TEXT NULL;
