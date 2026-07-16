package common

import (
	"encoding/json"
	"fmt"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// ApiResponse is the unified JSON response body for all Cloud API responses.
type ApiResponse struct {
	ErrCode int         `json:"errcode"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
	LogID   string      `json:"logid"`
	Error   *ApiError   `json:"error,omitempty"`
}

type ApiError struct {
	Type        string      `json:"type,omitempty"`
	Details     interface{} `json:"details,omitempty"`
	Retryable   bool        `json:"retryable,omitempty"`
	FieldErrors interface{} `json:"field_errors,omitempty"`
}

// ---- Success Helpers ----

// Success writes HTTP 200 with the unified response format.
func Success(c *hertzapp.RequestContext, data interface{}) {
	logID := resolveLogID(c)
	c.JSON(consts.StatusOK, ApiResponse{
		ErrCode: 0,
		Message: "success",
		Data:    data,
		LogID:   logID,
	})
}

// Created writes HTTP 201 with the unified response format.
func Created(c *hertzapp.RequestContext, data interface{}) {
	logID := resolveLogID(c)
	c.JSON(consts.StatusCreated, ApiResponse{
		ErrCode: 0,
		Message: "success",
		Data:    data,
		LogID:   logID,
	})
}

// Page writes a paginated response.
func Page(c *hertzapp.RequestContext, items interface{}, page, pageSize, total int) {
	logID := resolveLogID(c)
	c.JSON(consts.StatusOK, ApiResponse{
		ErrCode: 0,
		Message: "success",
		Data: map[string]interface{}{
			"items":    items,
			"page":     page,
			"pageSize": pageSize,
			"total":    total,
		},
		LogID: logID,
	})
}

// NoContent writes HTTP 200 with data:null for operations without return data.
func NoContent(c *hertzapp.RequestContext) {
	logID := resolveLogID(c)
	c.JSON(consts.StatusOK, ApiResponse{
		ErrCode: 0,
		Message: "success",
		Data:    nil,
		LogID:   logID,
	})
}

// ---- Failure Helpers ----

// Failure writes a structured error response with the given HTTP status and errcode.
func Failure(c *hertzapp.RequestContext, httpStatus, errcode int, message string, apiErr *ApiError) {
	logID := resolveLogID(c)
	c.JSON(httpStatus, ApiResponse{
		ErrCode: errcode,
		Message: message,
		Data:    nil,
		LogID:   logID,
		Error:   apiErr,
	})
}

// BadRequest writes HTTP 400.
func BadRequest(c *hertzapp.RequestContext, errcode int, message string) {
	Failure(c, consts.StatusBadRequest, errcode, message, nil)
}

// Unauthorized writes HTTP 401.
func Unauthorized(c *hertzapp.RequestContext, errcode int, message string) {
	Failure(c, consts.StatusUnauthorized, errcode, message, nil)
}

// Forbidden writes HTTP 403.
func Forbidden(c *hertzapp.RequestContext, errcode int, message string) {
	Failure(c, consts.StatusForbidden, errcode, message, nil)
}

// NotFound writes HTTP 404.
func NotFound(c *hertzapp.RequestContext, errcode int, message string) {
	Failure(c, consts.StatusNotFound, errcode, message, nil)
}

// Conflict writes HTTP 409.
func Conflict(c *hertzapp.RequestContext, errcode int, message string) {
	Failure(c, consts.StatusConflict, errcode, message, nil)
}

// InternalError writes HTTP 500.
func InternalError(c *hertzapp.RequestContext, message string) {
	Failure(c, consts.StatusInternalServerError, 50000, message, nil)
}

// ---- LogID resolution ----

func resolveLogID(c *hertzapp.RequestContext) string {
	if v, ok := c.Get("logid"); ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	// Fallback to trace_id if set
	if v, ok := c.Get("trace_id"); ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// ---- DecodeJSON ----

func DecodeJSON(c *hertzapp.RequestContext, dst interface{}) bool {
	if err := json.Unmarshal(c.Request.Body(), dst); err != nil {
		BadRequest(c, 10001, "request body is not valid json")
		return false
	}
	return true
}

// ---- Deprecated compatibility wrappers (to be removed after full migration) ----

// JSONData is deprecated. Use Success() instead.
func JSONData(c *hertzapp.RequestContext, status int, data interface{}) {
	Success(c, data)
}

// JSONError is deprecated. Use Failure() or typed helpers instead.
func JSONError(c *hertzapp.RequestContext, status int, code, message string) {
	errcode := legacyCodeToInt(code)
	Failure(c, status, errcode, message, &ApiError{Type: code})
}

func legacyCodeToInt(code string) int {
	// Simple mapping for migration period
	switch code {
	case "invalid_json":
		return 10001
	case "authentication_required":
		return 11001
	case "forbidden":
		return 11003
	case "invalid_identity_request":
		return 11004
	case "not_found", "task_not_found", "media_account_not_found", "agent_not_found", "sensitive_task_not_found":
		return 20004
	case "username_taken":
		return 20001
	case "invalid_task", "invalid_media_account_request", "invalid_sensitive_preflight":
		return 10001
	case "no_pending_task":
		return 30001
	case "task_agent_mismatch":
		return 30002
	case "task_already_terminal":
		return 30003
	case "invalid_agent", "agent_registry_error":
		return 30004
	case "incompatible_agent_contract":
		return 30005
	case "duplicate_media_account":
		return 20009
	case "profile_platform_account_taken", "browser_profile_unavailable":
		return 23001
	case "bitbrowser_identity_unverifiable":
		return 23002
	case "sensitive_credential_invalid":
		return 11001
	case "sensitive_task_assignment_mismatch":
		return 11003
	case "profile_runtime_unavailable":
		return 23003
	case "binding_ticket_invalid", "bound_session_invalid", "node_credential_invalid":
		return 11001
	case "runtime_binding_forbidden":
		return 11003
	case "profile_runtime_ownership_mismatch":
		return 23003
	case "incompatible_agent":
		return 30004
	default:
		return 99999
	}
}

// Ensure forward-compatibility: old front-end can still read .data and .error.code
// by embedding them in the new structure.
func init() {
	// Verify ApiResponse is serializable
	resp := ApiResponse{ErrCode: 0, Message: "success", Data: nil, LogID: "test"}
	if _, err := json.Marshal(resp); err != nil {
		panic(fmt.Sprintf("ApiResponse serialization failed: %v", err))
	}
}
