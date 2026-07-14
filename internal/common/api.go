package common

import (
	"encoding/json"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type DataResponse struct {
	Data interface{} `json:"data"`
}

func JSONData(c *hertzapp.RequestContext, status int, data interface{}) {
	c.JSON(status, DataResponse{Data: data})
}

func JSONError(c *hertzapp.RequestContext, status int, code, message string) {
	c.JSON(status, ErrorResponse{Error: ErrorBody{Code: code, Message: message}})
}

func DecodeJSON(c *hertzapp.RequestContext, dst interface{}) bool {
	if err := json.Unmarshal(c.Request.Body(), dst); err != nil {
		JSONError(c, consts.StatusBadRequest, "invalid_json", "request body is not valid json")
		return false
	}
	return true
}
