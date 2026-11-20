package contentpool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	contentservice "github.com/wt-media/wt-media-cloud/internal/modules/contentpool/service"
	"github.com/wt-media/wt-media-cloud/internal/shared/api"
)

func TestDiscoveryDependencyFailuresAreActionable(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{
		{"timeout", fmt.Errorf("search: %w", context.DeadlineExceeded), 504},
		{"not configured", contentservice.ErrCrawlerUnavailable, 503},
		{"upstream unavailable", fmt.Errorf("search: %w", contentservice.ErrDiscoveryUpstreamUnavailable), 502},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := hertzapp.NewContext(0)
			c.Set("logid", "tr-discovery-failure")
			writeDiscoveryError(c, tc.err)
			var body api.ApiResponse
			if err := json.Unmarshal(c.Response.Body(), &body); err != nil {
				t.Fatal(err)
			}
			if c.Response.StatusCode() != tc.status || body.LogID != "tr-discovery-failure" || body.Message == "挖掘服务内部错误" {
				t.Fatalf("status=%d body=%+v", c.Response.StatusCode(), body)
			}
			if body.Error == nil || body.Error.Retryable != (tc.status != 503) {
				t.Fatalf("dependency recovery missing: %+v", body.Error)
			}
		})
	}
}

func TestParseCategoryIDsAcceptsCommaSeparatedValues(t *testing.T) {
	got, ok := parseCategoryIDs(" 1, 3 ,,1 ")
	if !ok || len(got) != 2 || got[0] != 1 || got[1] != 3 {
		t.Fatalf("parseCategoryIDs() = %#v, %v", got, ok)
	}
	values := make([]string, 0, 51)
	for index := 0; index < 51; index++ {
		values = append(values, fmt.Sprintf("%d", index+1))
	}
	if _, ok := parseCategoryIDs(strings.Join(values, ",")); ok {
		t.Fatal("51 category ids should be rejected")
	}
}
