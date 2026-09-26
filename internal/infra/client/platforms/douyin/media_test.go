package douyin

import (
	"errors"
	"strings"
	"testing"
)

// The addresses below are what a payload really carries: the query string is the
// credential. Every host is under `example.invalid`, which RFC 6761 reserves so
// that it can never resolve — a fixture pointing at a real CDN could be fetched by
// accident, and a fixture quoting a real payload would put a credential in the
// repository.
const (
	signedAddress = "https://cdn.example.invalid/media/42.mp4?signature=DO-NOT-LOG-ME&expires=1700000000"
	otherAddress  = "https://backup.example.invalid/media/42.mp4?signature=DO-NOT-LOG-ME"
)

// payloadWith builds a payload shaped like the one the data API returns. It is
// assembled here rather than captured from a response: a captured payload is a
// credential with a video around it, and it expires.
func payloadWith(urlList ...any) map[string]any {
	return map[string]any{
		"aweme_id": "7123456789012345678",
		"desc":     "示例视频",
		"video": map[string]any{
			"play_addr": map[string]any{"url_list": urlList},
		},
	}
}

func TestMediaAddressReadsThePlayableAddress(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		address string
	}{
		{"the payload's address", signedAddress},
		// Character for character, not merely equivalent: the signature is in the
		// query string, so an address rebuilt from its parsed parts would verify as
		// something else. This is the case that shows the rebuilding — the scheme is
		// the one part `url.String()` would normalise.
		{"an upper case scheme", "HTTPS://cdn.example.invalid/media/42.mp4?signature=DO-NOT-LOG-ME"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := MediaAddress(payloadWith(testCase.address))
			if err != nil {
				t.Fatalf("MediaAddress() error = %v", err)
			}
			if got != testCase.address {
				t.Fatalf("address = %q, want the payload's address unchanged", got)
			}
		})
	}
}

func TestMediaAddressTakesTheFirstUsableAddressInTheList(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		urlList []any
		want    string
	}{
		{"one address", []any{signedAddress}, signedAddress},
		{"several equivalent addresses", []any{signedAddress, otherAddress}, signedAddress},
		// The list is a list of equivalents, so an entry that is not an address is a
		// hole in it rather than the answer.
		{"a blank entry before the address", []any{"", signedAddress}, signedAddress},
		{"whitespace before the address", []any{"   ", signedAddress}, signedAddress},
		{"an entry of the wrong type before the address", []any{42, signedAddress}, signedAddress},
		{"a null entry before the address", []any{nil, signedAddress}, signedAddress},
		{"surrounding whitespace is not part of the address", []any{"  " + signedAddress + "\n"}, signedAddress},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := MediaAddress(payloadWith(testCase.urlList...))
			if err != nil {
				t.Fatalf("MediaAddress() error = %v", err)
			}
			if got != testCase.want {
				t.Fatalf("address = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestMediaAddressRefusesAPayloadWithNoAddress(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		payload map[string]any
	}{
		{"no payload", nil},
		{"an empty payload", map[string]any{}},
		{"a video that is not an object", map[string]any{"video": "7123456789012345678"}},
		{"a video with no play address", map[string]any{"video": map[string]any{}}},
		{"a play address that is not an object", map[string]any{"video": map[string]any{"play_addr": "nope"}}},
		{"a play address with no list", map[string]any{"video": map[string]any{"play_addr": map[string]any{}}}},
		{"a list that is not an array", map[string]any{"video": map[string]any{"play_addr": map[string]any{"url_list": signedAddress}}}},
		// The wire never yields `[]string` for a field read into `any`, and accepting
		// a shape no payload produces would be a branch nothing keeps honest. It is
		// asserted here so that accepting one later is a decision rather than a slip.
		{"a list of Go strings rather than JSON values", map[string]any{"video": map[string]any{"play_addr": map[string]any{"url_list": []string{signedAddress}}}}},
		{"an empty list", payloadWith()},
		{"a blank entry", payloadWith("")},
		{"a whitespace entry", payloadWith("   ")},
		{"only entries that are not addresses", payloadWith(nil, 42)},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			address, err := MediaAddress(testCase.payload)
			if !errors.Is(err, ErrNoMediaAddress) {
				t.Fatalf("MediaAddress() = %q, %v, want ErrNoMediaAddress", address, err)
			}
			if address != "" {
				t.Fatalf("address = %q, want none alongside the refusal", address)
			}
		})
	}
}

// The fetch is an HTTP request, so an address naming another scheme is refused
// before anything tries to make one of it — and the refusal says nothing about the
// address, which is where a signed URL most easily escapes into a log.
func TestMediaAddressRefusesAnAddressItCannotFetch(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		address string
	}{
		{"another scheme", "ftp://cdn.example.invalid/media/42.mp4?signature=DO-NOT-LOG-ME"},
		{"a scheme that is not a protocol", "javascript:alert(DO-NOT-LOG-ME)"},
		{"no scheme", "cdn.example.invalid/media/42.mp4?signature=DO-NOT-LOG-ME"},
		{"a scheme relative address", "//cdn.example.invalid/media/42.mp4?signature=DO-NOT-LOG-ME"},
		{"no host", "https:///media/42.mp4?signature=DO-NOT-LOG-ME"},
		{"a host that cannot be parsed", "http://[::1/media/42.mp4?signature=DO-NOT-LOG-ME"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			address, err := MediaAddress(payloadWith(testCase.address))
			if !errors.Is(err, ErrNoMediaAddress) {
				t.Fatalf("MediaAddress() = %q, %v, want ErrNoMediaAddress", address, err)
			}
			if address != "" {
				t.Fatalf("address = %q, want none alongside the refusal", address)
			}
			for _, secret := range []string{"DO-NOT-LOG-ME", "example.invalid", "::1"} {
				if strings.Contains(err.Error(), secret) {
					t.Fatalf("error carries part of the address (%q): %v", secret, err)
				}
			}
		})
	}
}

// The envelope is asserted rather than searched: `postForm` decodes the response
// into the caller's map with no shape of its own, so this is the only place that
// says where a detail response keeps its item. A test that accepted
// `data.aweme_detail` too would make this package unable to tell which endpoint
// answered it, which is what `FindAuthor`'s different envelope exists to signal.
func TestDetailItemReadsTheItemOutOfADetailResponse(t *testing.T) {
	item, err := DetailItem(map[string]any{"data": payloadWith(signedAddress)})
	if err != nil {
		t.Fatalf("DetailItem() error = %v", err)
	}
	address, err := MediaAddress(item)
	if err != nil {
		t.Fatalf("MediaAddress() on the item error = %v", err)
	}
	if address != signedAddress {
		t.Fatalf("address = %q, want %q", address, signedAddress)
	}
}

// The envelope another endpoint answers with is not caught here, and this is the
// test that says so: `DetailItem` returns `data` without inspecting it, so a
// response from `FindAuthor` comes back as an item that has no video in it. The
// refusal happens one step later, in `MediaAddress`.
//
// It is asserted rather than assumed because the alternative — teaching
// `DetailItem` to recognise `aweme_detail` — would make this package unable to
// tell which endpoint answered it, and the difference is exactly what that
// nesting carries.
func TestDetailItemHandsTheWrongEnvelopeOnToBeRefusedByTheAddress(t *testing.T) {
	item, err := DetailItem(map[string]any{"data": map[string]any{"aweme_detail": payloadWith(signedAddress)}})
	if err != nil {
		t.Fatalf("DetailItem() error = %v, want the non-object refusal only", err)
	}
	address, err := MediaAddress(item)
	if !errors.Is(err, ErrNoMediaAddress) {
		t.Fatalf("MediaAddress() = %q, %v, want ErrNoMediaAddress", address, err)
	}
}

func TestDetailItemRefusesAResponseWithNoItem(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		payload map[string]any
	}{
		{"no payload", nil},
		{"an empty payload", map[string]any{}},
		{"a refusal carrying no data", map[string]any{"code": 40001, "msg": "invalid request"}},
		{"data that is not an object", map[string]any{"data": "no video"}},
		{"data that is null", map[string]any{"data": nil}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			item, err := DetailItem(testCase.payload)
			if !errors.Is(err, ErrNoDetailItem) {
				t.Fatalf("DetailItem() = %v, %v, want ErrNoDetailItem", item, err)
			}
			if item != nil {
				t.Fatalf("item = %v, want none alongside the refusal", item)
			}
		})
	}
}
