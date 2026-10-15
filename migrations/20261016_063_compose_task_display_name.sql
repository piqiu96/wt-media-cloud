-- A creation-time label. Existing tasks remain blank because current material
-- and strategy names cannot reconstruct their historical names reliably.
ALTER TABLE compose_tasks
    ADD COLUMN display_name VARCHAR(255) NOT NULL DEFAULT '' AFTER id;
