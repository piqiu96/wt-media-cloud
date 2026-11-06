# Storage migration

Uses the normal Cloud config and credentials. Apply database migrations first.

```
go run ./cmd/storage-migrate -inventory -report /tmp/storage-inventory.json
go run ./cmd/storage-migrate -report /tmp/storage-check.json
go run ./cmd/storage-migrate -apply -report /tmp/storage-migration.json
```

Inventory only lists database references. The default checks source bytes and
recorded size/SHA256 without uploading or changing references. Apply downloads
to a local temporary file, uploads the allocated ID path, reads it back, then
compares and switches each owner reference. Existing different content is a
conflict. Formal paths have permanent database content claims, including across
processes; replacements need a new identity. Old objects are never removed.

Failures are reported individually and cause nonzero exit. No failed reference
is switched; rerun after restoring old-bucket read access or resolving the
reported conflict. Reports contain internal object locations; keep them local.
