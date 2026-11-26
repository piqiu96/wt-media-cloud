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

- `v1/publications.openapi.yaml`: owner-only publication claim, unified report (keepalive/progress/pre-submit check) and report-safe original result. Revision `2026.11.22.2` adds independently frozen portrait cover grants and matching progress/result evidence; matching Agent and Desktop must be deployed together (ADR-0036). Revision `2026.11.22.1` adds owner-only due original-submission check claims and independent revision/generation-bound observations; public work is recorded atomically without mutating the original attempt. Agent routes use the singular `/api/v1/cloud-agent/publication-task` prefix.

## Publication admission revision 2026.11.25.2

Revision `2026.11.25.2` remains the minimum Baidu Agent revision; the current
provider revision is listed in the Bilibili section below. Only a current session with the capacity-bearing request
can obtain a new publish or check claim or resume an original scene. Older
publication report and completion packets retain original-attempt ReportSafe
settlement, original claim inspection/replay and result acknowledgement. A
fresh submit permission requires current-contract admission or a validated
current-contract continuation of that attempt. An older awaiting-human report
alone does not release automatic capacity.

Publish and check claim use the same persisted actual Profile admission. The
Agent records the exact check request before sending, then records the active
reservation and frozen input before processing. It records a possible-open
barrier before requesting a browser and durable closure evidence before sending
settlement. Recovery keeps the original key and settlement after Desktop
session replacement. A replay of a released key retrieves only its original acknowledgement. If the
response is lost before a reservation is persisted locally, the original
Agent sends `settle_unreceived: true` with the durable exact request key.
For this operation and known-reservation release, it uses a current Strict
credential for the same owner, including a replacement session, or a valid
non-current ReportSafe credential matching the original owner and admitted
session. An admitted key settles only its original reservation. An unadmitted
key records a cancellation fence and blocks a racing late claim. The Agent
durably chooses between accepting the active reply for work and settling the
unreceived request with a compare-and-set on the original key and phase.
Once it chooses settlement, a late active reply can only recover the original
acknowledgement.
Both outcomes return acknowledgement only. A current observation still
requires strict authentication and explicit, consistent closure evidence.
The separate release-only payload settles only the exact original reservation
under ReportSafe and cannot apply a public result.

### Legacy cutover

1. Close publication new-work admission before enabling the new claim paths.
   Stop old Cloud claim writers and drain old Agent publication checks and
   browser work. Keep original attempts' ReportSafe settlement available.
2. In the isolated preflight and production migration, retain every old
   running/awaiting-human attempt and unresolved interrupted attempt as an
   active automatic and window holder. Backfill actual Profile identity from
   the frozen original input. Duplicate or unprovable active identity blocks
   migration and requires reconciliation.
3. Legacy check generations have no new Profile-use holder. Read old check and
   original-attempt history to identify affected actual Profiles, then verify
   old Agent/check execution stopped and the original windows closed before
   admitting new work on each affected Profile. A generation, observation
   timeout or 24-hour schedule cutoff is not closure evidence. Keep any
   unresolved Profile blocked, or leave new publication admission closed
   until the environment safety check succeeds.
4. Open admission under the new version only after these checks. If the
   application is rolled back after v2 work started, drain or settle its
   Profile holders before restoring a worker that does not enforce them.

Admission and completion serialize batch, actual Profile, task and attempt
rows in that order where present. Discover identities before taking those locks,
then recheck them in the transaction. Retry only database races that committed no
authorization and executed no external action.

## Retired

- `v1/task-lease.openapi.yaml` and `v1/task-report.openapi.yaml`: the generic Cloud task table and endpoints are removed. File transfer uses its own contract.
- `v1/execution-preflight.openapi.yaml` and `v1/sensitive-profile-guard.openapi.yaml`: sensitive permit/task admission is removed with the generic task model.
- `v1/runtime-binding.openapi.yaml`: the device/node binding and secret-safe runtime report model is gone; runtime facts now travel with each admission request.
- `v1/registration-heartbeat.openapi.yaml`: unauthenticated agent registration and heartbeat are gone; the Agent proves itself with the Agent Token.


## Bilibili capability revision 2026.11.26.2

The provider revision is `2026.11.26.2`; the minimum Baidu consumer remains
`2026.11.25.2`. Publish/check admission accepts `supported_platforms`, omitted
as Baidu-only, and filters both fixed-group preparation and task selection.
Only an explicitly capable consumer receives new Bilibili work. New Bilibili
inputs freeze the platform and the independent space-cover digest/size; reports
and completions must match that digest. Baidu canonical input JSON and original
ReportSafe settlement remain unchanged. Deployment availability remains closed
by default until the complete live acceptance is verified.
- `v1/work-metrics.openapi.yaml`: additive owner-scoped six-counter public Bilibili collection; strict claims, report-safe lease results, 90-second leases, hourly success interval and failure backoff. No CK, browser scene or publication mutation. Deploy matching Local/Sidecar support for automatic/manual requests; old or offline Agents leave refresh queued.
