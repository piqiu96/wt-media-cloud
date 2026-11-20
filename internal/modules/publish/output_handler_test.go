package publish

import (
	app "github.com/cloudwego/hertz/pkg/app"
	"testing"
)

func TestOutputCandidateQueryParsing(t *testing.T) {
	c := app.NewContext(0)
	c.Request.SetRequestURI("/api/v1/publication/output?platform=baidu&page=8&page_size=10&search=source&category_id=9007199254740993")
	in, err := parseOutputCandidateQuery(c)
	if err != nil || in.Page != 8 || in.PageSize != 10 || in.Search != "source" || in.Platform != "baidu" || in.CategoryID != 9007199254740993 {
		t.Fatalf("query lost: %+v %v", in, err)
	}
	for _, query := range []string{"page=0", "page=-1", "page=bad", "page_size=101", "category_id=0", "category_id=overflow"} {
		c.Request.SetRequestURI("/api/v1/publication/output?platform=baidu&" + query)
		if _, err := parseOutputCandidateQuery(c); err == nil {
			t.Fatalf("invalid query accepted: %s", query)
		}
	}
}
