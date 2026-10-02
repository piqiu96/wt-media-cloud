-- Execution-layer node decoupling (CHG-20261002-074 stage 2).
-- local_agent_nodes.session_id tied node registration to one login session:
-- session invalidation (logout, replacement, expiry) revoked the node's
-- credential and made execution depend on a session that outlives its use.
-- The node now binds by device_id — the users.device_id device binding — so
-- credential validity is "device bound + node online", independent of sessions
-- (contract v2: node = execution-presence layer, session = identity layer).
-- Registering still gates on a live session through the binding ticket's own
-- session_id (local_agent_binding_tickets), which is untouched; only the node
-- row loses the session link. InnoDB drops the FK's auto-created index on
-- session_id along with the constraint, so no explicit DROP INDEX is needed
-- (the same shape migration 017 uses).
ALTER TABLE local_agent_nodes
  DROP FOREIGN KEY fk_local_agent_node_session,
  DROP COLUMN session_id;
