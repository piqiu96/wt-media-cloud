# WT Media Cloud

## Responsibility

Cloud owns formal business facts, permissions, tasks, platform configuration, audit logs, analytics read models, and Cloud-provided contracts.

## Structure

- `cmd/server`: process entrypoint.
- `internal/app`: dependency assembly, routing, lifecycle.
- `internal/infra`: database, object storage, logging, and scheduler placeholders.
- `internal/common`: shared scaffold helpers.
- `contracts`: Cloud-owned contract placeholder directories. Formal definitions are introduced by later CHGs.
- `web`: unified business Web source.

## Rules

- Handlers do not run SQL directly.
- Business modules do not mutate other modules' tables directly.
- Analytics is read-only.
- Long-running external work belongs to Agent tasks.
- Do not add microservices, MQ, or dynamic workflow engines in the scaffold phase.
