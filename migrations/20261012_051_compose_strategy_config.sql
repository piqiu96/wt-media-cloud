-- Align the dormant compose_strategies columns with ADR-0021: the strategy's
-- own settings are one schema-owned `config` (validated by the bound template
-- version's schema_json); the old model's separate allowed-overrides list has
-- no owner anymore — the per-template schema already declares what is
-- overridable. Strategy read/write endpoints are absent until M4-C4, so this
-- is a pure structure correction with no live readers.

ALTER TABLE compose_strategies
    RENAME COLUMN parameters_json TO config_json,
    DROP COLUMN allowed_overrides_json;
