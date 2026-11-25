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

- `v1/publication.openapi.yaml`: Revision `2026.11.25.2` enforces batch control on standalone task actions and adds associated batch, row and execution-group fields. Prior retained capabilities include separate validated 16:9 / 3:4 cover files and frozen render references (ADR-0036), complete single-row and multi-row DeepSeek suggestions over SSE with separate field adoption, original creation-result lookup, publish-owned production output candidates with platform occupancy, formal single-task preparation, scoped read-back, revision-checked edits and separate execution authorization/attempt history, four-conclusion result verification and reviewing supplement, exact receipt/public identity separation (ADR-0035), canonical public-work association, safe same-task retry, unclaimed cancellation retaining occupancy, verified failure abandonment releasing occupancy, immutable template cover input checks, and recent original-submission observation details.

- `v1/work.openapi.yaml`: Permission-scoped read-only work list/detail and actual-publication-time summary, backed by platform-work identity deduplication.

- `v1/image.openapi.yaml`: CHG106 singular image-asset registration, scoped queries, revision-checked maintenance and category-first selection; files retain provider-owned upload and access.

- `v1/cover-template.openapi.yaml`: Template maintenance, bounded actual rendering from authorized image/output backgrounds, and frozen formal cover read-back for draft recovery.

- `v1/publication-batch.openapi.yaml` provides CHG107 batch resource reads, allocation, readiness, saved candidate configuration, revision-checked editing, conditional copy and operation records.
- Publication batch revision `2026.11.25.2` adds first-start preview and atomic authorization, fixed-group control, revision-bound pause/resume/stop/finish, formal task pagination and verified public counts. The existing configuration save remains preparation.

- Publication batch revision `2026.11.25.3` adds the optional A02 conflict detail `start_request_applied: false` only when a different durable first-start key and valid immutable receipt prove this pending request did not win. Other conflicts omit it; the original key/digest remain private. Cloud-Agent and publish error-code revisions remain `2026.11.25.2`.

CHG109 publishes `v1/publication-plan.openapi.yaml` revision `2026.11.25.4` and advances publication/publication-batch projections to the same Cloud API revision. Plans orchestrate own prepared batches using the existing batch start transaction, fixed selections and 10% handoff; no makeup or cross-user authorization API. Local plan continuation is Agent-owned LocalAPI `2026.11.25.2`. Cloud-Agent and publishing error revisions remain `2026.11.25.2`.
