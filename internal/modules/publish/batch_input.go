package publish

import (
	"bytes"
	"encoding/json"

	"github.com/wt-media/wt-media-cloud/internal/modules/publish/service"
)

// Go's JSON decoder deliberately treats an omitted field and null as zero on
// value types. Keep those distinct from explicit zero in the batch protocol.
func requireBatchFields(body map[string]json.RawMessage, names ...string) error {
	for _, name := range names {
		raw, exists := body[name]
		if !exists || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return service.ErrInvalid
		}
	}
	return nil
}

func requireBatchObjects(raw json.RawMessage, names ...string) error {
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return service.ErrInvalid
	}
	for _, item := range items {
		if err := requireBatchFields(item, names...); err != nil {
			return err
		}
	}
	return nil
}

func checkBatchAllocationFields(body map[string]json.RawMessage) error {
	for _, spec := range []struct {
		list   string
		fields []string
	}{
		{"accounts", []string{"media_account_id", "limit"}},
		{"output_ids", nil},
		{"manual_pairs", []string{"category_id", "media_account_id", "composite_output_id"}},
	} {
		if _, present := body[spec.list]; !present {
			continue
		}
		if err := requireBatchFields(body, spec.list); err != nil {
			return err
		}
		if spec.fields != nil {
			if err := requireBatchObjects(body[spec.list], spec.fields...); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkBatchRequestFields(raw []byte, destination any) error {
	var body map[string]json.RawMessage
	switch destination.(type) {
	case *service.CreateBatchInput, *service.EditBatchInput, *service.ValidateBatchInput, *service.BatchAllocationInput:
		if err := json.Unmarshal(raw, &body); err != nil {
			return service.ErrInvalid
		}
	default:
		return nil
	}
	if err := requireBatchFields(body, "platform", "targets"); err != nil {
		return err
	}
	if err := requireBatchObjects(body["targets"], "category_id", "target_count"); err != nil {
		return err
	}
	switch input := destination.(type) {
	case *service.BatchAllocationInput:
		if err := requireBatchFields(body, "accounts", "output_ids"); err != nil {
			return err
		}
		return checkBatchAllocationFields(body)
	case *service.CreateBatchInput:
		if err := requireBatchFields(body, "name", "request_key"); err != nil {
			return err
		}
	case *service.EditBatchInput:
		if err := requireBatchFields(body, "name", "expected_revision"); err != nil {
			return err
		}
	case *service.ValidateBatchInput:
		_, hasID := body["batch_id"]
		_, hasRevision := body["expected_revision"]
		if (hasID || hasRevision) && (!hasID || !hasRevision || input.BatchID == 0 || input.ExpectedRevision == 0) {
			return service.ErrInvalid
		}
	}
	if err := requireBatchFields(body, "config", "execution_setting"); err != nil {
		return err
	}
	var config, settings map[string]json.RawMessage
	if json.Unmarshal(body["config"], &config) != nil || json.Unmarshal(body["execution_setting"], &settings) != nil {
		return service.ErrInvalid
	}
	if err := requireBatchFields(settings, "concurrency_limit", "group_interval_seconds", "between_group_policy", "exception_task_limit"); err != nil {
		return err
	}
	if err := requireBatchFields(config, "candidates"); err != nil {
		return err
	}
	if err := checkBatchAllocationFields(config); err != nil {
		return err
	}
	return requireBatchObjects(config["candidates"], "row_key", "input_revision", "field_revisions", "category_id", "media_account_id", "composite_output_id", "order", "content")
}
