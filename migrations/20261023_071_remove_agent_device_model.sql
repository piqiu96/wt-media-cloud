-- The Agent is not a device, a node or a business resource. This migration
-- removes the durable state the fixed-device model kept for it: the per-user
-- device identity, the registered node rows and their presence/binding tables,
-- and the node columns on the sensitive-operation tables. Authentication is now
-- the session-bound Agent Token, which needs no row.
--
-- This is deliberately destructive and must ship with the same release as the
-- matching Desktop and Agent versions: an old Agent has no authentication path
-- left and must fail closed rather than fall back to a node credential.
--
-- Before applying to a non-empty environment, record the row counts that are
-- about to be dropped:
--   SELECT COUNT(*) FROM local_agent_nodes;
--   SELECT COUNT(*) FROM local_agent_binding_tickets;
--   SELECT COUNT(*) FROM browser_profile_runtime_presence;
--   SELECT COUNT(*) FROM agent_nodes;
--   SELECT COUNT(*) FROM users WHERE device_id IS NOT NULL;
--   SELECT COUNT(*) FROM file_transfer_tasks WHERE assigned_node_id IS NOT NULL;
--   SELECT COUNT(*) FROM sensitive_browser_tasks WHERE node_id IS NOT NULL;
--   SELECT COUNT(*) FROM sensitive_profile_permits WHERE node_id IS NOT NULL;
-- Pending and running file-transfer rows stay claimable: the user-scoped claim
-- picks them up by `requested_by`, so nothing has to be rewritten or re-created.

-- Child foreign keys and columns first: MySQL refuses to drop a column that a
-- foreign key still uses, and refuses to drop the parent table while children
-- reference it.
ALTER TABLE sensitive_profile_permits
    DROP FOREIGN KEY fk_sensitive_profile_permit_node,
    DROP INDEX uq_sensitive_profile_permit_credential_hash,
    DROP COLUMN node_id,
    DROP COLUMN credential_hash;

ALTER TABLE sensitive_browser_tasks
    DROP FOREIGN KEY fk_sensitive_browser_task_node,
    DROP INDEX idx_sensitive_browser_tasks_node_status,
    DROP COLUMN node_id;

DROP TABLE browser_profile_runtime_presence;
DROP TABLE local_agent_binding_tickets;
DROP TABLE agent_nodes;
DROP TABLE local_agent_nodes;

ALTER TABLE users
    DROP COLUMN device_public_key,
    DROP COLUMN device_name,
    DROP COLUMN device_bound_at,
    DROP COLUMN device_last_verified_at,
    DROP COLUMN device_id;

-- A transfer belongs to the user who requested it. `claimed_by` is an audit
-- label for whichever process took the lease, and `attempt_id` binds every
-- continuation report to that claim.
ALTER TABLE file_transfer_tasks
    DROP INDEX idx_file_transfer_tasks_assigned_node,
    DROP COLUMN assigned_node_id,
    DROP COLUMN assigned_device_id,
    CHANGE COLUMN claimed_by_node_id claimed_by VARCHAR(128) NULL,
    ADD COLUMN attempt_id VARCHAR(64) NULL AFTER claimed_by,
    ADD KEY idx_file_transfer_tasks_requested (requested_by, status, created_at);
