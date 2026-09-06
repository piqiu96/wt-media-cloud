-- M2-C Task 9: persist the operator-selected proxy source while keeping the
-- supplier extraction URL out of ordinary proxy projections.
ALTER TABLE proxy_configs
    ADD COLUMN source_type VARCHAR(16) NOT NULL DEFAULT 'static' AFTER id,
    ADD COLUMN extract_url TEXT NULL AFTER supplier;

-- Existing rows are canonical static records. Rollback (if required before
-- code deployment) drops these two additive columns only.
