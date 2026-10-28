# Cloud-Agent API Contracts

Cloud-owned contracts used by Local/Cloud Agent processes when communicating with Cloud.

Authentication is a session-bound Agent Token (`/api/v1/local-agent/agent-token`) in two
modes: **strict** for new operations (the token must still be the one bound to the current
desktop session) and **report-safe** for finishing already claimed work (a replaced
desktop session does not interrupt it). A user session is not accepted on these routes,
and an Agent Token is not accepted on user business API.

## Active Contracts

- `v1/compatibility.openapi.yaml`: runtime compatibility endpoint.
- `v1/agent-token.openapi.yaml`: exchanging a Desktop session for a signed Agent Token.
- `v1/file-transfer.openapi.yaml`: Local Agent lease, heartbeat, progress, completion, and cancellation-observation endpoints for `user_download` transfer tasks.

## Retired

- `v1/task-lease.openapi.yaml` and `v1/task-report.openapi.yaml`: the generic Cloud task table and endpoints are removed. File transfer uses its own contract.
- `v1/execution-preflight.openapi.yaml` and `v1/sensitive-profile-guard.openapi.yaml`: sensitive permit/task admission is removed with the generic task model.
- `v1/runtime-binding.openapi.yaml`: the device/node binding and secret-safe runtime report model is gone; runtime facts now travel with each admission request.
- `v1/registration-heartbeat.openapi.yaml`: unauthenticated agent registration and heartbeat are gone; the Agent proves itself with the Agent Token.
