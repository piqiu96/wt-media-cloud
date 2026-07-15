# Cloud Migrations

MySQL migration files belong here.

M0 intentionally does not define business tables. Future CHGs add migrations with their owning contracts and acceptance evidence.

Use `WT_MEDIA_MYSQL_DSN` for the Cloud process database connection and migration command:

```text
WT_MEDIA_MYSQL_DSN='user:password@tcp(127.0.0.1:3306)/wt_media_cloud?parseTime=true&loc=UTC' scripts/migrate.sh
```

The command creates the DSN database when missing, applies files in lexical order, records applied versions in `schema_migrations`, and skips already applied migrations on repeat runs.

## Active Migrations

- `20260714_001_identity.sql`: M2-C1 users, game scopes, single-session records, and audit logs.
- `20260714_002_media_accounts.sql`: M2-C2 user/game-owned media accounts, identification/status fields, secret Cookie storage, and simple account tags.
- `20260714_003_browser_profiles.sql`: M2-C3 BitBrowser user binding, staged Profile scans, confirmed Profile mirrors, and media-account Profile reference integrity.
- `20260714_004_agent_runtime.sql`: M2-C4 session-bound local Agent nodes, secret-safe environment facts, and Profile runtime presence.
- `20260714_005_sensitive_profile_locks.sql`: M2-C5 pre-authorized sensitive browser tasks and non-reusable Profile permits.
