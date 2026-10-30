package identity

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

// ProxyID is numeric on the wire. Internal services may still pass its decimal
// representation as a string to existing SQL and lock helpers.
type ProxyID string

func (id ProxyID) MarshalJSON() ([]byte, error) {
	if id == "" {
		return []byte("null"), nil
	}
	value, err := strconv.ParseUint(string(id), 10, 64)
	if err != nil || value == 0 {
		return nil, fmt.Errorf("invalid numeric proxy id %q", id)
	}
	return []byte(strconv.FormatUint(value, 10)), nil
}

func (id *ProxyID) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) {
		*id = ""
		return nil
	}
	if len(data) > 1 && data[0] == '"' {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		data = []byte(text)
	}
	value, err := strconv.ParseUint(string(data), 10, 64)
	if err != nil || value == 0 {
		return fmt.Errorf("invalid numeric proxy id %q", data)
	}
	*id = ProxyID(strconv.FormatUint(value, 10))
	return nil
}
