// Package dto owns runtime-binding input and output contracts.
package dto

import (
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding/model"
)

type BindingTicketGrant struct {
	TicketID     string    `json:"ticket_id"`
	BindingToken string    `json:"binding_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type RegisterLocalInput struct {
	BindingToken         string `json:"binding_token"`
	AgentID              string `json:"agent_id"`
	DeviceID             string `json:"device_id"`
	DevicePublicKey      string `json:"device_public_key"`
	DeviceSignature      string `json:"device_signature"`
	DeviceName           string `json:"device_name"`
	BindDevice           bool   `json:"bind_device"`
	AgentVersion         string `json:"agent_version"`
	ContractMajorVersion string `json:"contract_major_version"`
	ContractRevision     string `json:"contract_revision"`
}

type Registration struct {
	Node           model.AgentNode `json:"node"`
	NodeCredential string          `json:"node_credential"`
}

type RuntimeReport struct {
	OperatingSystem  string               `json:"operating_system"`
	CPUArchitecture  string               `json:"cpu_architecture"`
	AgentVersion     string               `json:"agent_version"`
	PythonVersion    string               `json:"python_version"`
	FFmpeg           model.DependencyFact `json:"ffmpeg"`
	WorkdirStatus    string               `json:"workdir_status"`
	Disk             model.DiskFact       `json:"disk"`
	BitBrowserStatus string               `json:"bitbrowser_status"`
	MainUserID       string               `json:"main_user_id,omitempty"`
	BitProfileIDs    []string             `json:"bit_profile_ids,omitempty"`
}
