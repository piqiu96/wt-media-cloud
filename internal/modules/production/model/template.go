package model

import "encoding/json"

type ComposeTemplate struct {
	ID         int64           `json:"id"`
	Code       string          `json:"code"`
	Name       string          `json:"name"`
	Version    uint32          `json:"version"`
	Schema     json.RawMessage `json:"schema"`
	Status     string          `json:"status"`
	ExecutorID string          `json:"-"`
}
