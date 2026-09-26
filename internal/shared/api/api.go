// Package api defines the shared Cloud HTTP response envelope and helpers.
package api

import (
	"encoding/json"

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

// Page writes a paginated api.
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

// Accepted writes HTTP 202 with the unified response format.
//
// 202 rather than 201 for a queued transfer: the request created a durable task,
// but nothing has been transferred yet. A 201 would read as "the download
// exists", which is the one thing this API must not imply before an executor has
// produced bytes.
func Accepted(c *hertzapp.RequestContext, data interface{}) {
	logID := resolveLogID(c)
	c.JSON(consts.StatusAccepted, ApiResponse{
		ErrCode: 0,
		Message: "success",
		Data:    data,
		LogID:   logID,
	})
}

// NoContentEmpty writes HTTP 204 with no body.
//
// Deliberately not `NoContent`, which answers 200 with a `data:null` envelope:
// the frontend's `parseResponse` turns that envelope into a returned value, so
// the two are observably different to a caller. Only the one frozen contract that
// says 204 uses this (`DELETE /api/v1/material-usages/{usage_id}`); every other
// "nothing to return" endpoint keeps `NoContent` at 200, because changing them
// would silently rewrite twelve unrelated contracts.
func NoContentEmpty(c *hertzapp.RequestContext) {
	c.SetStatusCode(consts.StatusNoContent)
}

// UnprocessableEntity writes HTTP 422 with the given frozen error name.
//
// The name is required rather than optional: 422 is reserved here for integrity
// failures, and the caller distinguishing them needs the name the contract
// publishes, not a numeric errcode that `contracts/cloud-error-codes/` never
// declares.
func UnprocessableEntity(c *hertzapp.RequestContext, errcode int, message, errorType string) {
	FailureNamed(c, consts.StatusUnprocessableEntity, errcode, message, errorType)
}

// ConflictNamed writes HTTP 409 carrying the frozen error name.
//
// Two distinct 409s reach this API — "the material has no prepared source yet"
// and "there is no fresh Local Agent node" — and a caller holding only the
// frozen name list cannot tell them apart by errcode. The name travels in
// `error.type`.
func ConflictNamed(c *hertzapp.RequestContext, errcode int, message, errorType string) {
	FailureNamed(c, consts.StatusConflict, errcode, message, errorType)
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

// FailureNamed writes a structured error response whose `error.type` carries the
// frozen error name from `contracts/cloud-error-codes/`.
//
// It exists because the frozen error contracts publish names and HTTP statuses
// only — no numeric errcode — so a client that wants to branch on *which* failure
// it got has exactly this field to read. `Failure` leaves `error` nil, which
// means "no machine-readable discriminator"; this is the variant for the cases
// where there is one.
func FailureNamed(c *hertzapp.RequestContext, httpStatus, errcode int, message, errorType string) {
	Failure(c, httpStatus, errcode, message, &ApiError{Type: errorType})
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
