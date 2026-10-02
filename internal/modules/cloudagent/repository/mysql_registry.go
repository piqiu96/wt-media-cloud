package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/model"
	"gorm.io/gorm"
)

func RegisterAgent(req RegisterAgentInput) (model.AgentNode, error) {
	return registerAgent(database.DB(), req)
}

func registerAgent(db *gorm.DB, req RegisterAgentInput) (model.AgentNode, error) {
	now := time.Now().UTC()
	capabilities, _ := json.Marshal(req.Capabilities)
	if err := db.Exec(`INSERT INTO agent_nodes (agent_id, mode, version, contract_major_version, contract_revision,
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
		now, now, string(capabilities),
	).Error; err != nil {
		return model.AgentNode{}, err
	}
	return getAgent(db, req.AgentID)
}

func Heartbeat(agentID string, req HeartbeatInput) (model.AgentNode, error) {
	return heartbeat(database.DB(), agentID, req)
}

// heartbeat is pure presence: it updates the registry row and nothing else.
// The old code additionally drained and replaced a local node when its bound
// session was invalidated — contract v1, where session death revoked execution.
// Under contract v2 the node is the execution-presence layer bound by device,
// not by a login session, so a logged-out/replaced/expired session must not
// revoke it (runtimebinding.authenticateCredential is the same split). Node
// revocation now happens only where it belongs: unbinding the device or a
// superseding registration for the same device.
func heartbeat(db *gorm.DB, agentID string, req HeartbeatInput) (model.AgentNode, error) {
	status := req.Status
	if status == "" {
		status = "online"
	}

	result := db.Exec(`UPDATE agent_nodes SET status = ?, last_heartbeat_at = ? WHERE agent_id = ?`, status, time.Now().UTC(), agentID)
	if result.Error != nil {
		return model.AgentNode{}, result.Error
	}
	if result.RowsAffected == 0 {
		return model.AgentNode{}, model.ErrAgentNotFound
	}
	return getAgent(db, agentID)
}

func GetAgent(agentID string) (model.AgentNode, error) {
	return getAgent(database.DB(), agentID)
}

func getAgent(db *gorm.DB, agentID string) (model.AgentNode, error) {
	var node model.AgentNode
	var capabilities string
	err := db.Raw(`SELECT agent_id, mode, version, contract_major_version,
		contract_revision, status, registered_at, last_heartbeat_at, COALESCE(capabilities,'[]')
		FROM agent_nodes WHERE agent_id = ?`, agentID).Row().Scan(
		&node.AgentID, &node.Mode, &node.Version,
		&node.ContractMajorVersion, &node.ContractRevision,
		&node.Status, &node.RegisteredAt, &node.LastHeartbeatAt, &capabilities,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return model.AgentNode{}, model.ErrAgentNotFound
	}
	if err != nil {
		return model.AgentNode{}, fmt.Errorf("scan agent: %w", err)
	}
	_ = json.Unmarshal([]byte(capabilities), &node.Capabilities)
	if node.Capabilities == nil {
		node.Capabilities = []string{}
	}
	return node, nil
}
