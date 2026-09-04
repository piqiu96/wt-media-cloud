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
- `20260814_024_media_account_tags_unique_account_scope.sql`: restores the media-account tag uniqueness key to `(user_id, media_account_id, tag_name)` after migration 022 removed and re-added `media_account_id`.
- `20260903_025_media_account_games.sql`: moves the media-account game association into the canonical `media_account_games` relation table and removes `media_accounts.game_id`.

## Migration 025 Safety

Migration 025 changes only the media-account-to-game association. It creates
`media_account_games`, clears only that newly created relation table, copies
every non-empty legacy `media_accounts.game_id`, then removes the legacy index
and column. Media accounts, tags, Cookie fields, check items and Profile
bindings are not deleted or rewritten.

Before applying to a non-empty environment, record the counts of media
accounts, tags, non-null Cookie/check-item fields and Profile bindings; also
verify that every non-empty legacy game ID exists in `operation_games` and is
enabled. After applying, verify `schema_migrations` contains version
`20260903_025_media_account_games`, relation-row count equals the pre-migration
non-empty legacy count, `SHOW COLUMNS FROM media_accounts` has no `game_id`, and
the composite primary key rejects duplicate `(media_account_id, game_id)` rows.

Rollback is not lossless after one account has more than one relation row. A
recovery can add a nullable legacy column and project a chosen relation value,
but it must first archive every relation row and never silently discard the
additional associations.

## Migration 024 Safety

Migration 024 changes only the unique index on `media_account_tags`; it does not rewrite tag rows. The migration runner makes it repeat-safe by recording the version in `schema_migrations`. It restores the intended rule that the same label may be reused by different accounts owned by one user.

Before applying it to a non-empty environment, check for duplicate `(user_id, media_account_id, tag_name)` rows. After applying it, verify the index column order with `SHOW INDEX FROM media_account_tags`. A rollback may restore the former `(user_id, tag_name)` key only after duplicate tag names across accounts have been resolved; otherwise the rollback would discard a supported relationship or fail to create the key.

## M2-A1 Migration Safety

MySQL DDL statements can auto-commit even when the migration runner opens a transaction. `20260722_012_user_team_model.sql` therefore uses shadow columns and validates every required mapping before it drops an old user reference.

Before applying this migration to a non-empty environment:

1. take a restorable database backup;
2. record row counts for `users` and every table with a `user_id` or `actor_user_id`;
3. apply the migration to a copy of the database first;
4. verify that every numeric user reference resolves to `users.id` and that the row counts are unchanged.

The original string user identifier remains in `users.legacy_id`. It is the recovery mapping source if an operator must rebuild the old string relationships. A rollback must recreate shadow string columns from `users.legacy_id`, validate all mappings, swap the foreign keys in reverse order, and only then restore the old primary key. Never drop the numeric columns or `legacy_id` before that validation succeeds.
