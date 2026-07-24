# Cloud Migrations

MySQL migration files belong here.

M0 intentionally does not define business tables. Future CHGs add migrations with their owning contracts and acceptance evidence.

Use `WT_MEDIA_MYSQL_DSN` for the Cloud process database connection and migration command:

```text
scripts/migrate.sh
```

Local scripts default to the stable local database `wt_media_cloud`. Do not create one database per CHG/Task during local acceptance; apply schema changes to the stable local database through repeat-safe migrations.

The command creates the DSN database when missing, applies files in lexical order, records applied versions in `schema_migrations`, and skips already applied migrations on repeat runs.

## Active Migrations

- `20260714_001_identity.sql`: M2-C1 users, game scopes, single-session records, and audit logs.
- `20260714_002_media_accounts.sql`: M2-C2 user/game-owned media accounts, identification/status fields, secret Cookie storage, and simple account tags.
- `20260714_003_browser_profiles.sql`: M2-C3 BitBrowser user binding, staged Profile scans, confirmed Profile mirrors, and media-account Profile reference integrity.
- `20260714_004_agent_runtime.sql`: M2-C4 session-bound local Agent nodes, secret-safe environment facts, and Profile runtime presence.
- `20260714_005_sensitive_profile_locks.sql`: M2-C5 pre-authorized sensitive browser tasks and non-reusable Profile permits.
- `20260722_012_user_team_model.sql`: M2-A1 numeric user identifiers, fixed roles, operation teams, historical team snapshots, and all existing user foreign-key migration.

## M2-A1 Migration Safety

MySQL DDL statements can auto-commit even when the migration runner opens a transaction. `20260722_012_user_team_model.sql` therefore uses shadow columns and validates every required mapping before it drops an old user reference.

Before applying this migration to a non-empty environment:

1. take a restorable database backup;
2. record row counts for `users` and every table with a `user_id` or `actor_user_id`;
3. apply the migration to a copy of the database first;
4. verify that every numeric user reference resolves to `users.id` and that the row counts are unchanged.

The original string user identifier remains in `users.legacy_id`. It is the recovery mapping source if an operator must rebuild the old string relationships. A rollback must recreate shadow string columns from `users.legacy_id`, validate all mappings, swap the foreign keys in reverse order, and only then restore the old primary key. Never drop the numeric columns or `legacy_id` before that validation succeeds.
