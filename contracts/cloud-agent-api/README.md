# Cloud-Agent API Contracts

Cloud-owned contracts used by Local/Cloud Agent processes when communicating with Cloud.

## Active Contracts

- `v1/compatibility.openapi.yaml`: minimal runtime compatibility endpoint for M1-C1.
- `v1/registration-heartbeat.openapi.yaml`: Agent registration and heartbeat endpoints for M1-C2.
- `v1/task-lease.openapi.yaml`: M1-C3 noop task creation, claim, and lease endpoints.
- `v1/task-report.openapi.yaml`: M1-C4 task status report endpoint.

## Not Active Yet

- Task polling.
- Retry or recovery behavior.
