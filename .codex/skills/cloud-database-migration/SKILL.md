---
name: cloud-database-migration
description: Add or review Cloud MySQL migrations and related module boundaries.
---
<!-- GENERATED FILE - DO NOT EDIT DIRECTLY -->
<!-- Source: skills/cloud/cloud-database-migration/SKILL.md -->


# Cloud Database Migration

Use this skill when changing Cloud schema or migration files.

## Check

- Migration belongs in `wt-media-cloud/migrations`.
- Business module ownership is clear.
- Analytics remains read-only.
- Handler code does not run SQL directly.
- Backward compatibility and rollback notes are captured.
- Tests or verification queries are listed.

## Output

Summarize affected tables, module owner, migration direction, and verification.
