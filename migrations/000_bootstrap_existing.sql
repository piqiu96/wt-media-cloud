-- Bootstrap: Record existing migrations as already applied.
-- This migration must sort before 20260714_001 to ensure the runner
-- skips re-applying migrations whose tables already exist.
-- Safe to re-run: uses INSERT IGNORE and only records a version when the
-- corresponding legacy migration table already exists.

INSERT IGNORE INTO schema_migrations (version, name, applied_at)
SELECT '20260714_001_identity', 'identity', UTC_TIMESTAMP(6)
WHERE EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'users');
INSERT IGNORE INTO schema_migrations (version, name, applied_at)
SELECT '20260714_002_media_accounts', 'media_accounts', UTC_TIMESTAMP(6)
WHERE EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'media_accounts');
INSERT IGNORE INTO schema_migrations (version, name, applied_at)
SELECT '20260714_003_browser_profiles', 'browser_profiles', UTC_TIMESTAMP(6)
WHERE EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'browser_profiles');
INSERT IGNORE INTO schema_migrations (version, name, applied_at)
SELECT '20260714_004_agent_runtime', 'agent_runtime', UTC_TIMESTAMP(6)
WHERE EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'local_agent_binding_tickets');
INSERT IGNORE INTO schema_migrations (version, name, applied_at)
SELECT '20260714_005_sensitive_profile_locks', 'sensitive_profile_locks', UTC_TIMESTAMP(6)
WHERE EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'sensitive_browser_tasks');
