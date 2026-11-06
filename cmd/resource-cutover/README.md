# Resource cutover

Explicit one-time maintenance for CHG-20261106-125, after authorization to delete old data. The default inventories only; `-apply` deletes the selected database rows. It never deletes remote objects.

```sh
go run ./cmd/resource-cutover -backup /private/new-inventory.json
go run ./cmd/resource-cutover -apply -backup /private/new-before-delete.json
```

Backups contain complete affected rows and source-content pointers, use mode 0600, refuse overwrite, and sync contents and the parent directory before deletion. Keep them private. Selection uses nonempty old locations outside `s3://wt-media/` and the explicitly retired template codes. Unprepared materials without an old object reference are retained.

A single transaction locks selected records, refuses current-bucket collateral or unfinished old composition tasks/running transfers, writes the backup, clears source-content material pointers, then deletes dependents with foreign keys enabled. Deleted row counts must match the snapshot; any failure rolls back. New current assets, operations files, the slice library, users and unrelated source-content metadata remain. Re-run inventories an empty retired set.

Before operating elsewhere run the gated local rollback test with `WT_MEDIA_TEST_LIVE_CUTOVER=1 go test ./internal/modules/production/repository -run TestLiveLegacyCutoverRollbackAndCurrentAssetProtection -count=1`.
