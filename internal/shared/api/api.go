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
