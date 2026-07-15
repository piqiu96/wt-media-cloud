-- Bootstrap: Record existing migrations as already applied.
-- This migration must sort before 20260714_001 to ensure the runner
-- skips re-applying migrations whose tables already exist.
-- Safe to re-run: uses INSERT IGNORE.

INSERT IGNORE INTO schema_migrations (version, name, applied_at) VALUES
('20260714_001_identity', 'identity', UTC_TIMESTAMP(6));
INSERT IGNORE INTO schema_migrations (version, name, applied_at) VALUES
('20260714_002_media_accounts', 'media_accounts', UTC_TIMESTAMP(6));
INSERT IGNORE INTO schema_migrations (version, name, applied_at) VALUES
('20260714_003_browser_profiles', 'browser_profiles', UTC_TIMESTAMP(6));
INSERT IGNORE INTO schema_migrations (version, name, applied_at) VALUES
('20260714_004_agent_runtime', 'agent_runtime', UTC_TIMESTAMP(6));
INSERT IGNORE INTO schema_migrations (version, name, applied_at) VALUES
('20260714_005_sensitive_profile_locks', 'sensitive_profile_locks', UTC_TIMESTAMP(6));
