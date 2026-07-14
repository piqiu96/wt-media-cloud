# Cloud-Agent API Contracts

Cloud-owned contracts used by Local/Cloud Agent processes when communicating with Cloud.

## Active Contracts

- `v1/compatibility.openapi.yaml`: minimal runtime compatibility endpoint for M1-C1.
- `v1/registration-heartbeat.openapi.yaml`: Agent registration and heartbeat endpoints for M1-C2.
- `v1/task-lease.openapi.yaml`: M1-C3 noop task creation, claim, and lease endpoints.

## Not Active Yet

- Task polling.
- Progress and result upload.
- Retry or recovery behavior.
