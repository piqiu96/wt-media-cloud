-- CHG-20261012-081 walkthrough amendment: strategies no longer carry a
-- content_type. What a strategy configures is whatever its bound template
-- version's schema declares, so a separate free-text type on the strategy
-- row has no owner and no reader.

ALTER TABLE compose_strategies
    DROP COLUMN content_type;
