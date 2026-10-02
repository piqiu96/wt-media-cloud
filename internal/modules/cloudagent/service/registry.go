package service

import (
	"sync"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/model"
)

const (
	AgentStatusOnline   = "online"
	AgentStatusDraining = "draining"
)

var (
	ErrAgentNotFound     = model.ErrAgentNotFound
	ErrInvalidAgent      = model.ErrInvalidAgent
	ErrIncompatibleAgent = model.ErrIncompatibleAgent
)

type (
	RegisterAgentRequest = dto.RegisterAgentRequest
	HeartbeatRequest     = dto.HeartbeatRequest
	AgentNode            = model.AgentNode
)

type Registry struct {
	mu     sync.Mutex
	now    func() time.Time
	agents map[string]AgentNode
}

func NewRegistry() *Registry {
	return &Registry{
		now:    func() time.Time { return time.Now().UTC() },
		agents: make(map[string]AgentNode),
	}
}

func NewRegistryWithClock(now func() time.Time) *Registry {
	r := NewRegistry()
	r.now = now
	return r
}

func (r *Registry) Register(req RegisterAgentRequest) (AgentNode, error) {
	if req.AgentID == "" || req.Version == "" || !validMode(req.Mode) {
		return AgentNode{}, ErrInvalidAgent
	}
	if !IsAgentCompatible(req.ContractMajorVersion, req.ContractRevision) {
		return AgentNode{}, ErrIncompatibleAgent
	}

	now := r.now().Format(time.RFC3339)
	node := AgentNode{
		AgentID:              req.AgentID,
		Mode:                 req.Mode,
		Version:              req.Version,
		ContractMajorVersion: req.ContractMajorVersion,
		ContractRevision:     req.ContractRevision,
		Status:               AgentStatusOnline,
		RegisteredAt:         now,
		LastHeartbeatAt:      now,
		Capabilities:         append([]string(nil), req.Capabilities...),
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.agents[req.AgentID]; ok {
		node.RegisteredAt = existing.RegisteredAt
	}
	r.agents[req.AgentID] = node
	return node, nil
}

func (r *Registry) Heartbeat(agentID string, req HeartbeatRequest) (AgentNode, error) {
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

	r.mu.Lock()
	defer r.mu.Unlock()
	node, ok := r.agents[agentID]
	if !ok {
		return AgentNode{}, ErrAgentNotFound
	}
	node.Status = status
	node.LastHeartbeatAt = r.now().Format(time.RFC3339)
	r.agents[agentID] = node
	return node, nil
}

func (r *Registry) Get(agentID string) (AgentNode, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	node, ok := r.agents[agentID]
	if !ok {
		return AgentNode{}, ErrAgentNotFound
	}
	return node, nil
}

func validMode(mode string) bool {
	return mode == "local" || mode == "cloud"
}

func validStatus(status string) bool {
	return status == AgentStatusOnline || status == AgentStatusDraining
}
