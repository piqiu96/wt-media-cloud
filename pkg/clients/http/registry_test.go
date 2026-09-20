package httpclient_test

import (
	"testing"

	httpclient "github.com/wt-media/wt-media-cloud/pkg/clients/http"
	pkgconfig "github.com/wt-media/wt-media-cloud/pkg/config"
)

func TestInitializePublishesFilenameDerivedClientsAndCloseRevokesThem(t *testing.T) {
	closer, err := httpclient.Initialize([]pkgconfig.Document{
		document("agent", `
timeout = "1s"
[connection]
dial_timeout = "100ms"
[retry]
attempts = 1
`),
		document("douyin", `
timeout = "2s"
[connection]
dial_timeout = "100ms"
[retry]
attempts = 1
`),
	}, nil)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	if httpclient.Get("agent") == nil || httpclient.Get("douyin") == nil {
		t.Fatal("Initialize did not publish both clients")
	}
	if httpclient.Get("agent") == httpclient.Get("douyin") {
		t.Fatal("Initialize published the same client twice")
	}
	if err := closer(); err != nil {
		t.Fatalf("closer() error = %v", err)
	}
	assertGetPanics(t, "agent")
}

func TestInitializeRejectsDuplicatesNonTOMLAndInvalidConfigs(t *testing.T) {
	tests := [][]pkgconfig.Document{
		{document("same", valid()), document("same", valid())},
		{{Name: "json", Format: pkgconfig.FormatJSON, Raw: []byte(`{"timeout":"1s"}`)}},
		{document("invalid", "timeout = \"1s\"\n[retry]\nattempts = 0\n")},
	}
	for index, documents := range tests {
		closer, err := httpclient.Initialize(documents, nil)
		if err == nil {
			t.Fatalf("tests[%d] Initialize() error = nil", index)
		}
		if closer != nil {
			_ = closer()
		}
		assertGetPanics(t, "test")
	}
}

func TestInitializeRollsBackEarlierClientsOnLaterFailure(t *testing.T) {
	closer, err := httpclient.Initialize([]pkgconfig.Document{
		document("valid", valid()),
		document("invalid", "timeout = \"1s\"\n[retry]\nattempts = 0\n"),
	}, nil)
	if err == nil || closer != nil {
		t.Fatalf("Initialize() closer=%t error=%v", closer != nil, err)
	}
	assertGetPanics(t, "valid")
}

func document(name, content string) pkgconfig.Document {
	return pkgconfig.Document{Name: name, Format: pkgconfig.FormatTOML, Raw: []byte(content)}
}

func valid() string {
	return "timeout = \"1s\"\n[connection]\ndial_timeout = \"100ms\"\n[retry]\nattempts = 1\n"
}

func assertGetPanics(t *testing.T, name string) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("Get(%q) did not panic after close or failed initialization", name)
		}
	}()
	_ = httpclient.Get(name)
}
