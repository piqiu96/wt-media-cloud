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
- `v1/execution-preflight.openapi.yaml`: the single admission decision for local operations (11 checks).
- `v1/sensitive-profile-guard.openapi.yaml`: sensitive Profile preflight; permits are a per-Profile mutex, not a credential.
- `v1/task-lease.openapi.yaml`: user-scoped claim, attempt issuance, and task reads.
- `v1/task-report.openapi.yaml`: progress, activity, terminal result (idempotent, stale-attempt aware), and user retry/cancel.
- `v1/file-transfer.openapi.yaml`: Local Agent lease, heartbeat, progress, completion, and cancellation-observation endpoints for `user_download` transfer tasks.

## Retired

- `v1/runtime-binding.openapi.yaml`: the device/node binding and secret-safe runtime report model is gone; runtime facts now travel with each admission request.
- `v1/registration-heartbeat.openapi.yaml`: unauthenticated agent registration and heartbeat are gone; the Agent proves itself with the Agent Token.
