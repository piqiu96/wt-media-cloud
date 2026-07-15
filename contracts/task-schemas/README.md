# Task Schemas

Cloud-owned universal task schema contract.

## Status: Active (M1-R1)

Formal task schema is active as of revision `2026.07.15.1`.

### Contents

- `v1/task-schema.openapi.yaml` — Universal task model: status state machine, task types,
  request/response DTOs, and error code enumeration.

### Consumers

- `wt-media-agent` — locks task schema version for executor compatibility.
- `wt-media-desktop` — locks task schema version for display compatibility.

### Revision History

| Revision | Date | Changes |
|---|---|---|
| 2026.07.15.1 | 2026-07-15 | First active revision (M1-R1). |
