# Cloud Error Codes

Cloud-owned API and task intake error code definitions.

## Active Definitions

- `v1/identity.yaml`: M2-C1 authentication and authorization errors.
- `v1/media-account.yaml`: M2-C2 validation, visibility, duplicate, and store errors.
- `v1/browser-profile.yaml`: M2-C3 scan identity, expiry, Profile availability, and store errors.
- `v1/agent-runtime.yaml`: Agent Token authentication errors.
- `v1/proxy.yaml`: proxy reservation, quota, availability, and stale-operation errors.
- `v1/content-production.yaml`: content discovery dependency failures, M4-A material visibility and material-usage errors.
- `v1/file-transfer.yaml`: M4-A transfer lifecycle, local-node, and integrity errors.

- `v1/publish.yaml`: Publication preparation errors; Go constants live in `internal/shared/errcode/publish.go`.
