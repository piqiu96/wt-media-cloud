-- Internal optimistic concurrency token; report_seq remains the Agent protocol sequence.
-- Existing attempts start at revision 1, without changing their state or evidence.
ALTER TABLE publication_attempt ADD COLUMN revision BIGINT UNSIGNED NOT NULL DEFAULT 1;
