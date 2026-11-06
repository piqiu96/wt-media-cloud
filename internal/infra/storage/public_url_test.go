package storage

import (
	"net/url"
	"testing"
)

func TestPublicObjectURLUsesPersistedLocationAndEscapesOnce(t *testing.T) {
	ref := (&url.URL{Scheme: "s3", Host: "wt-media", Path: "/file-ku/files/12_开头 100%#1.png"}).String()
	got, err := PublicObjectURL("https://wt-media.bucket.example/", "wt-media", ref)
	want := "https://wt-media.bucket.example/file-ku/files/12_%E5%BC%80%E5%A4%B4%20100%25%231.png"
	if err != nil || got != want {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestPublicObjectURLRefusesUnboundOrAmbiguousLocations(t *testing.T) {
	for _, tc := range []struct{ base, bucket, ref string }{
		{"", "wt-media", "s3://wt-media/file-ku/files/1.png"},
		{"http://bucket.example", "wt-media", "s3://wt-media/file-ku/files/1.png"},
		{"https://user:pass@bucket.example", "wt-media", "s3://wt-media/file-ku/files/1.png"},
		{"https://bucket.example?token=x", "wt-media", "s3://wt-media/file-ku/files/1.png"},
		{"https://bucket.example/path", "wt-media", "s3://wt-media/file-ku/files/1.png"},
		{"https://bucket.example", "wt-media", "s3://old/file-ku/files/1.png"},
		{"https://bucket.example", "wt-media", "file-ku/files/1.png"},
		{"https://bucket.example", "wt-media", "s3://wt-media/file-ku/files/../1.png"},
		{"https://bucket.example", "wt-media", "s3://wt-media/file-ku/files/1.png?x=y"},
		{"https://bucket.example", "wt-media", "s3://wt-media/file-ku/files/%2e%2e/1.png"},
	} {
		if got, err := PublicObjectURL(tc.base, tc.bucket, tc.ref); err == nil {
			t.Fatalf("accepted %+v as %s", tc, got)
		}
	}
}
