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
INSERT IGNORE INTO schema_migrations (version, name, applied_at)
SELECT '20260715_006_tasks', 'tasks', UTC_TIMESTAMP(6)
WHERE EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'tasks');
INSERT IGNORE INTO schema_migrations (version, name, applied_at)
SELECT '20260715_007_agent_registry', 'agent_registry', UTC_TIMESTAMP(6)
WHERE EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'agent_nodes');
INSERT IGNORE INTO schema_migrations (version, name, applied_at)
SELECT '20260716_008_proxy_configs', 'proxy_configs', UTC_TIMESTAMP(6)
WHERE EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'proxy_configs')
  AND EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'proxy_platform_quotas');
INSERT IGNORE INTO schema_migrations (version, name, applied_at)
SELECT '20260721_009_task_payload', 'task_payload', UTC_TIMESTAMP(6)
WHERE EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'tasks' AND column_name = 'payload_json');
INSERT IGNORE INTO schema_migrations (version, name, applied_at)
SELECT '20260721_010_profile_proxy_fields', 'profile_proxy_fields', UTC_TIMESTAMP(6)
WHERE EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'browser_profiles' AND column_name = 'proxy_type')
  AND EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'browser_profiles' AND column_name = 'proxy_host')
  AND EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'browser_profiles' AND column_name = 'proxy_port')
  AND EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'browser_profiles' AND column_name = 'remark');
INSERT IGNORE INTO schema_migrations (version, name, applied_at)
SELECT '20260723_014_profile_sync_candidate_runtime_fields', 'profile_sync_candidate_runtime_fields', UTC_TIMESTAMP(6)
WHERE EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'profile_sync_candidates' AND column_name = 'proxy_type')
  AND EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'profile_sync_candidates' AND column_name = 'proxy_host')
  AND EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'profile_sync_candidates' AND column_name = 'proxy_port')
  AND EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'profile_sync_candidates' AND column_name = 'remark');
INSERT IGNORE INTO schema_migrations (version, name, applied_at)
SELECT '20260721_011_task_result', 'task_result', UTC_TIMESTAMP(6)
WHERE EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'tasks' AND column_name = 'result_json');
