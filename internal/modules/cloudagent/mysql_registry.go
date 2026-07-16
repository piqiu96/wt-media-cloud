package cloudagent

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// MySQLRegistry persists agent registrations in MySQL. Falls back to in-memory when db is nil.
type MySQLRegistry struct {
	db  *sql.DB
	now func() time.Time
	mem *Registry // fallback when db is nil
}

func NewMySQLRegistry(db *sql.DB) *MySQLRegistry {
	return &MySQLRegistry{
		db:  db,
		now: func() time.Time { return time.Now().UTC() },
		mem: NewRegistry(),
	}
}

func (r *MySQLRegistry) Register(req RegisterAgentRequest) (AgentNode, error) {
	if r.db == nil {
		return r.mem.Register(req)
	}
	if req.AgentID == "" || req.Version == "" || !validMode(req.Mode) {
		return AgentNode{}, ErrInvalidAgent
	}
	if !IsAgentCompatible(req.ContractMajorVersion, req.ContractRevision) {
		return AgentNode{}, ErrIncompatibleAgent
	}

	now := r.now()
	caps, _ := json.Marshal(req.Capabilities)

	_, err := r.db.Exec(
		`INSERT INTO agent_nodes (agent_id, mode, version, contract_major_version, contract_revision,
		                          status, registered_at, last_heartbeat_at, capabilities)
		 VALUES (?, ?, ?, ?, ?, 'online', ?, ?, ?)
		 ON DUPLICATE KEY UPDATE
		     mode = VALUES(mode),
		     version = VALUES(version),
		     contract_major_version = VALUES(contract_major_version),
		     contract_revision = VALUES(contract_revision),
		     status = 'online',
		     last_heartbeat_at = VALUES(last_heartbeat_at),
		     capabilities = VALUES(capabilities)`,
		req.AgentID, req.Mode, req.Version, req.ContractMajorVersion, req.ContractRevision,
		now, now, string(caps),
	)
	if err != nil {
		return r.mem.Register(req)
	}
	return r.scanAgent(r.db.QueryRow(`SELECT agent_id, mode, version, contract_major_version,
		contract_revision, status, registered_at, last_heartbeat_at, COALESCE(capabilities,'[]')
		FROM agent_nodes WHERE agent_id = ?`, req.AgentID))
}

func (r *MySQLRegistry) Heartbeat(agentID string, req HeartbeatRequest) (AgentNode, error) {
	if r.db == nil {
		return r.mem.Heartbeat(agentID, req)
	}
	if agentID == "" {
		return AgentNode{}, ErrInvalidAgent
	}
	status := req.Status
	if status == "" {
		status = AgentStatusOnline
	}
	if !validStatus(status) {
		return AgentNode{}, ErrInvalidAgent
	}

	now := r.now()

	// Check session validity for local agents: if the bound session was invalidated,
	// reject the heartbeat so the agent stops claiming new tasks.
	if status == AgentStatusOnline {
		var sessionValid bool
		err := r.db.QueryRow(
			`SELECT COUNT(*) > 0 FROM local_agent_nodes n
			 JOIN user_sessions s ON n.session_id = s.id
			 WHERE n.agent_id = ? AND n.mode = 'local' AND s.invalidated_at IS NULL`,
			agentID,
		).Scan(&sessionValid)
		if err == nil && !sessionValid {
			r.db.Exec(`UPDATE agent_nodes SET status = 'draining' WHERE agent_id = ?`, agentID)
			r.db.Exec(`UPDATE local_agent_nodes SET status = 'replaced' WHERE agent_id = ?`, agentID)
			return AgentNode{}, ErrSessionInvalid
		}
	}

	result, err := r.db.Exec(
		`UPDATE agent_nodes SET status = ?, last_heartbeat_at = ? WHERE agent_id = ?`,
		status, now, agentID,
	)
	if err != nil {
		return r.mem.Heartbeat(agentID, req)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return AgentNode{}, ErrAgentNotFound
	}
	return r.Get(agentID)
}

func (r *MySQLRegistry) Get(agentID string) (AgentNode, error) {
	if r.db == nil {
		return r.mem.Get(agentID)
	}
	return r.scanAgent(r.db.QueryRow(`SELECT agent_id, mode, version, contract_major_version,
		contract_revision, status, registered_at, last_heartbeat_at, COALESCE(capabilities,'[]')
		FROM agent_nodes WHERE agent_id = ?`, agentID))
}

func (r *MySQLRegistry) scanAgent(row *sql.Row) (AgentNode, error) {
	var n AgentNode
	var capsJSON string
	err := row.Scan(
		&n.AgentID, &n.Mode, &n.Version,
		&n.ContractMajorVersion, &n.ContractRevision,
		&n.Status, &n.RegisteredAt, &n.LastHeartbeatAt,
		&capsJSON,
	)
	if err == sql.ErrNoRows {
		return AgentNode{}, ErrAgentNotFound
	}
	if err != nil {
		return AgentNode{}, fmt.Errorf("scan agent: %w", err)
	}
	json.Unmarshal([]byte(capsJSON), &n.Capabilities)
	if n.Capabilities == nil {
		n.Capabilities = []string{}
	}
	return n, nil
}
