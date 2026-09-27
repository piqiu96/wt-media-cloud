// Package dto owns the Cloud file-transfer request and response bodies.
//
// The types here are the wire contract and nothing else: they declare no
// behaviour, so the mapping from `model` stays in the module that owns the
// response. What they do carry is the key set, which is frozen in
// `contracts/business-schemas/v1/file-transfer.yaml` for the session API and in
// `contracts/cloud-agent-api/v1/file-transfer.openapi.yaml` for the executor
// API, and `dto_test.go` reads both files to check it.
package dto

import "time"

// Task is the session-facing body of one transfer task, and its key set is
// exactly the properties of `FileTransferTask` in
// `business-schemas/v1/file-transfer.yaml`.
//
// No key is optional in the encoding. The contract declares a property *set*, and
// the frontend reads the body as one: a pending task whose size is not known yet
// must answer `total_bytes: 0` rather than omit the key, because a missing key and
// a zero are different things to a reader that indexes into the object. That is
// also why the nullable properties are pointers — `estimated_remaining_seconds`
// has no zero-valued representation of "not known yet", and the schema says
// `nullable: true` for it.
//
// Nothing here is an address. `local_path`, `download_url`, `storage_credential`
// and `node_credential` are `forbidden_properties` in the contract: a signed URL
// appears only in a lease, is never persisted, and never reaches this body.
//
// `asset_title` was added for the download centre, which otherwise had only an id to
// name a row with: `file_name` is the executor's report and does not exist until the
// transfer finishes, and a list of `#42` is not something an operator can act on.
type Task struct {
	ID                        string    `json:"id"`
	AssetType                 string    `json:"asset_type"`
	AssetID                   int64     `json:"asset_id"`
	AssetTitle                string    `json:"asset_title"`
	Purpose                   string    `json:"purpose"`
	ExecutionScope            string    `json:"execution_scope"`
	Status                    string    `json:"status"`
	TotalBytes                int64     `json:"total_bytes"`
	CompletedBytes            int64     `json:"completed_bytes"`
	BytesPerSecond            int64     `json:"bytes_per_second"`
	EstimatedRemainingSeconds *int64    `json:"estimated_remaining_seconds"`
	AttemptCount              int       `json:"attempt_count"`
	MaxAttempts               int       `json:"max_attempts"`
	ChecksumSHA256            *string   `json:"checksum_sha256"`
	FileName                  *string   `json:"file_name"`
	ErrorCode                 *string   `json:"error_code"`
	ErrorMessage              *string   `json:"error_message"`
	CreatedAt                 time.Time `json:"created_at"`
	UpdatedAt                 time.Time `json:"updated_at"`
}

// LocalLease is what a claiming executor receives, and the only place a download
// grant appears in the whole contract.
//
// `expected_sha256` is a plain string here because the schema requires it: Cloud
// only creates a local transfer for a material whose source object was already
// verified, so `ready` and `video_sha256` are written together and the executor
// always has something to check its bytes against.
type LocalLease struct {
	TaskID               string    `json:"task_id"`
	AssetType            string    `json:"asset_type"`
	AssetID              int64     `json:"asset_id"`
	Title                string    `json:"title"`
	TotalBytes           int64     `json:"total_bytes"`
	ExpectedSHA256       string    `json:"expected_sha256"`
	MaxAttempts          int       `json:"max_attempts"`
	AttemptCount         int       `json:"attempt_count"`
	LeaseSeconds         int       `json:"lease_seconds"`
	DownloadURL          string    `json:"download_url"`
	DownloadURLExpiresAt time.Time `json:"download_url_expires_at"`
}

// ClaimResult answers a claim. `Task` is emitted even when it is nil: the schema
// requires the key and makes it explicitly nullable, so "there is nothing to do"
// is a null task rather than a missing key or a 200 read as an error.
type ClaimResult struct {
	Task *LocalLease `json:"task"`
}

// TransferTerminal echoes the outcome Cloud recorded, so an executor can confirm
// what it produced instead of assuming it.
//
// `file_name` is omitted when there is none rather than sent as an empty string
// or a null: the schema declares it as a plain (non-nullable) string, so null
// would be invalid, and the key is not required.
type TransferTerminal struct {
	TaskID         string `json:"task_id"`
	Status         string `json:"status"`
	CompletedBytes int64  `json:"completed_bytes"`
	FileName       string `json:"file_name,omitempty"`
}

// HeartbeatRequest renews the lease and carries how far the executor has got, for
// the UI's benefit. `additionalProperties: false` in the contract means this
// struct is the whole body — an extra field is a rejected request, not a
// harmless addition.
type HeartbeatRequest struct {
	CompletedBytes int64 `json:"completed_bytes"`
}

// ProgressRequest reports transfer progress. Progress does not renew the lease,
// which is why it carries no lease field and why the contract gives its response
// no body.
type ProgressRequest struct {
	CompletedBytes int64 `json:"completed_bytes"`
	BytesPerSecond int64 `json:"bytes_per_second"`
}

// CompletionRequest is the executor's terminal report.
//
// The contract makes `completed_bytes` and `sha256` conditional on
// `status: success` and `error_code` conditional on a failure, and the handler
// enforces both, because a success with nothing to check its bytes against is
// exactly what the integrity rule exists to refuse.
type CompletionRequest struct {
	Status         string `json:"status"`
	CompletedBytes int64  `json:"completed_bytes"`
	SHA256         string `json:"sha256"`
	FileName       string `json:"file_name"`
	ErrorCode      string `json:"error_code"`
	ErrorMessage   string `json:"error_message"`
}
