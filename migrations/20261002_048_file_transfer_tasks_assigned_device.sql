-- Dedupe dimension node → device (CHG-20261002-074 stage 3).
-- user_download dedupe_key was keyed on assigned_node_id. A node is a
-- registration instance: renewal and supersede mint a new node id, and unbind
-- replaces the device's node entirely — so the keyed dimension is not stable
-- across a device's lifetime, and two downloads on two devices would be told
-- apart only by accident of the generation counter. The device (users.device_id,
-- carried by the node at resolve time) is the stable destination: a device
-- switch is a new destination that must get its own download, while a repeat
-- click on the same device must keep collapsing onto the outstanding task. The
-- dedupe key now binds to assigned_device_id.
-- Backfill: existing rows take the device of the node they were assigned to
-- (local_agent_nodes.device_id). Rows whose node is gone keep NULL — they are
-- history, and a NULL device never matches a device-scoped check.
ALTER TABLE file_transfer_tasks ADD COLUMN assigned_device_id VARCHAR(128) NULL AFTER assigned_node_id;

UPDATE file_transfer_tasks t
  JOIN local_agent_nodes n ON n.id = t.assigned_node_id
  SET t.assigned_device_id = n.device_id
  WHERE t.assigned_device_id IS NULL AND t.assigned_node_id IS NOT NULL;
