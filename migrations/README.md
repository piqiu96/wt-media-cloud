# Cloud Migrations

MySQL migration files belong here.

M0 intentionally does not define business tables. Future CHGs add migrations with their owning contracts and acceptance evidence.

Use `WT_MEDIA_MYSQL_DSN` for the Cloud process database connection once SQL repositories are enabled.

## Active Migrations

- `20260714_001_identity.sql`: M2-C1 users, game scopes, single-session records, and audit logs.
