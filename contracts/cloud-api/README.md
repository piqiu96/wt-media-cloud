# Cloud API Contracts

Cloud-owned public HTTPS JSON API contracts for Web and Desktop consumers.

## Active Contracts

- `v1/identity.openapi.yaml`: M2-C1 login, current user, user administration, password, and session endpoints.
- `v1/media-accounts.openapi.yaml`: Media-account maintenance, read-only checks, credentials and Cookie updates; one-table batch imports with preview, serial grants, row recovery and shared window occupancy. Old read-sync endpoints are removed.
- `v1/browser-profiles.openapi.yaml`: M2-C3 staged BitBrowser scan, confirmation, and Profile list endpoints.
- `v1/proxies.openapi.yaml`: proxy ledger, unified capacity, acknowledged Profile assignment and read-back-based direct unbind.
- `v1/content-production.openapi.yaml`: Material library, user material usages, production tasks and verified compose outputs including actual final-video thumbnails.
- `v1/file-transfer.openapi.yaml`: M4-A executor-neutral transfer-task list and cancellation endpoints.
- `v1/files.openapi.yaml`: Independent operations image/video uploads, permanent public bucket URLs, and compatible Cloud preview/attachment URLs.

- `v1/publication.openapi.yaml`: Revision `2026.11.22.1`: separate validated 16:9 / 3:4 cover files and frozen render references (ADR-0036), complete single-row DeepSeek suggestions over SSE with separate field adoption, original creation-result lookup, publish-owned production output candidates with platform occupancy, formal single-task preparation, scoped read-back, revision-checked edits and separate execution authorization/attempt history, four-conclusion result verification and reviewing supplement, exact receipt/public identity separation (ADR-0035), canonical public-work association, safe same-task retry, unclaimed cancellation retaining occupancy, verified failure abandonment releasing occupancy, immutable template cover input checks, and recent original-submission observation details.

- `v1/work.openapi.yaml`: Permission-scoped read-only work list/detail and actual-publication-time summary, backed by platform-work identity deduplication.

- `v1/image.openapi.yaml`: CHG106 singular image-asset registration, scoped queries, revision-checked maintenance and category-first selection; files retain provider-owned upload and access.

- `v1/cover-template.openapi.yaml`: Template maintenance, bounded actual rendering from authorized image/output backgrounds, and frozen formal cover read-back for draft recovery.
