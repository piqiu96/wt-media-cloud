package image

import "testing"

func TestBatchCoversKeepPlatformSpecificSpecifications(t *testing.T) {
	for _, tc := range []struct {
		platform, spec string
		width, height  int
		valid          bool
	}{
		{"baidu", "landscape", 16, 9, true}, {"baidu", "portrait", 3, 4, true},
		{"bilibili", "recommendation", 4, 3, true}, {"bilibili", "space", 16, 9, true},
		{"bilibili", "portrait", 0, 0, false}, {"bilibili", "landscape", 0, 0, false},
		{"baidu", "recommendation", 0, 0, false}, {"xiaohongshu", "space", 0, 0, false},
	} {
		width, height, valid := batchCoverRatio(tc.platform, tc.spec)
		if width != tc.width || height != tc.height || valid != tc.valid {
			t.Fatalf("wrong ratio for %s/%s", tc.platform, tc.spec)
		}
		row := batchCoverRow{RowKey: "row", InputRevision: "1", FieldRevisions: map[string]string{"cover_file_id": "0"}, Platform: tc.platform, CategoryID: 1,
			Specs: []batchCoverSpec{{Specification: tc.spec, TemplateID: 1, PublishedRevision: 1, Title: "封面"}}}
		if validBatchCoverRow(row) != tc.valid {
			t.Fatalf("wrong admission for %s/%s", tc.platform, tc.spec)
		}
	}
}
