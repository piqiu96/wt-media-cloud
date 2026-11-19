package errcode_test

import (
	"github.com/wt-media/wt-media-cloud/internal/shared/errcode"
	"gopkg.in/yaml.v3"
	"os"
	"testing"
)

func TestPublishCodesMatchProviderContract(t *testing.T) {
	raw, err := os.ReadFile("../../../contracts/cloud-error-codes/v1/publish.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var contract struct {
		Errors map[string]struct {
			Code int `yaml:"errcode"`
		} `yaml:"errors"`
	}
	if err := yaml.Unmarshal(raw, &contract); err != nil {
		t.Fatal(err)
	}
	for name, code := range map[string]int{
		"publication_invalid": errcode.PublishInvalid, "publication_not_found": errcode.PublishNotFound,
		"publication_conflict": errcode.PublishConflict, "publication_output_occupied": errcode.PublishOutputOccupied,
		"publication_platform_closed": errcode.PublishPlatformClosed, "publication_unavailable": errcode.PublishUnavailable,
	} {
		if contract.Errors[name].Code != code {
			t.Errorf("%s contract=%d code=%d", name, contract.Errors[name].Code, code)
		}
	}
}
