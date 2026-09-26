package sourcefile

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func payload(size int) []byte {
	body := make([]byte, size)
	for index := range body {
		body[index] = byte('a' + index%26)
	}
	return body
}

// zeroModTime suppresses `Last-Modified`, so no test depends on the clock.
func zeroModTime() time.Time { return time.Time{} }

func server(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server, NewWithClient(server.Client())
}

func TestOpenAPositiveOffsetAsksForARange(t *testing.T) {
	var seen string
	server, client := server(t, func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("Range")
		http.ServeContent(w, r, "42.mp4", zeroModTime(), strings.NewReader(string(payload(1000))))
	})

	body, source, err := client.Open(context.Background(), server.URL+"/media/42.mp4?signature=x", 400)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer body.Close()
	if seen != "bytes=400-" {
		t.Fatalf("Range header = %q, want the offset asked for", seen)
	}
	got, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if len(got) != 600 || got[0] != payload(1000)[400] {
		t.Fatalf("body is %d bytes starting with %q, want the tail from offset 400", len(got), string(got[:1]))
	}
	if source.RangeStart != 400 {
		t.Fatalf("RangeStart = %d, want 400", source.RangeStart)
	}
	// The size that matters is the whole file's, not this fragment's: it is what a
	// progress bar divides by and what the integrity check compares against.
	if source.TotalBytes != 1000 {
		t.Fatalf("TotalBytes = %d, want the whole file rather than the fragment", source.TotalBytes)
	}
}

// A zero offset means "the whole file", and asking for `bytes=0-` would invite a
// 206 whose `Content-Range` a caller would then have to interpret for no reason.
func TestOpenAFreshOffsetSendsNoRangeHeader(t *testing.T) {
	var seen string
	server, client := server(t, func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("Range")
		http.ServeContent(w, r, "42.mp4", zeroModTime(), strings.NewReader(string(payload(10))))
	})

	body, source, err := client.Open(context.Background(), server.URL, 0)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer body.Close()
	if seen != "" {
		t.Fatalf("Range header = %q, want none for a fresh download", seen)
	}
	if source.RangeStart != 0 || source.TotalBytes != 10 {
		t.Fatalf("source = %+v, want the whole file from zero", source)
	}
}

// The failure this package exists to make visible: a server that ignores `Range`
// answers 200 with the entire file. A caller that assumed it had resumed would
// append the whole file to a partial one — a file of the right length and the
// wrong bytes, which only a hash check much later would notice.
func TestOpenReportsAServerThatIgnoredTheRange(t *testing.T) {
	server, client := server(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000")
		_, _ = w.Write(payload(1000))
	})

	body, source, err := client.Open(context.Background(), server.URL, 400)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer body.Close()
	if source.RangeStart != 0 {
		t.Fatalf("RangeStart = %d, want 0: the server sent the file from the beginning", source.RangeStart)
	}
	got, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if len(got) != 1000 {
		t.Fatalf("body is %d bytes, want the whole file", len(got))
	}
}

// A 206 whose window does not start where it was asked to. The status code says
// "partial" and the header says which part; only the header answers the question
// the caller asked.
func TestOpenBelievesContentRangeOverTheStatusCode(t *testing.T) {
	server, client := server(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Range", "bytes 100-999/1000")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(payload(900))
	})

	body, source, err := client.Open(context.Background(), server.URL, 400)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer body.Close()
	if source.RangeStart != 100 || source.TotalBytes != 1000 {
		t.Fatalf("source = %+v, want the window the header named", source)
	}
}

// A 206 with no usable `Content-Range` is a server saying "partial" without saying
// which part. The one offset it can be trusted to mean is the one that was asked
// for, and the total stays unknown rather than being filled in from a
// `Content-Length` that describes the fragment.
func TestOpenTreatsAnUnreadableContentRangeAsTheOffsetItAskedFor(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		header string
	}{
		{"absent", ""},
		{"not a range", "pages 1-2"},
		{"no total", "bytes 400-999"},
		{"not a number", "bytes four-999/1000"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			server, client := server(t, func(w http.ResponseWriter, r *http.Request) {
				if testCase.header != "" {
					w.Header().Set("Content-Range", testCase.header)
				}
				w.WriteHeader(http.StatusPartialContent)
				_, _ = w.Write(payload(600))
			})

			body, source, err := client.Open(context.Background(), server.URL, 400)
			if err != nil {
				t.Fatalf("Open() error = %v", err)
			}
			defer body.Close()
			if source.RangeStart != 400 {
				t.Fatalf("RangeStart = %d, want the offset that was asked for", source.RangeStart)
			}
			if source.TotalBytes != -1 {
				t.Fatalf("TotalBytes = %d, want -1 for an unknown total", source.TotalBytes)
			}
		})
	}
}

// `*` is the header's own way of saying the total is not known yet. It is a
// readable window, not a parse failure: the start is real and the caller needs it.
func TestOpenReadsAnOpenEndedContentRange(t *testing.T) {
	server, client := server(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Range", "bytes 400-999/*")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(payload(600))
	})

	body, source, err := client.Open(context.Background(), server.URL, 400)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer body.Close()
	if source.RangeStart != 400 || source.TotalBytes != -1 {
		t.Fatalf("source = %+v, want the window with an unknown total", source)
	}
}

// An unknown total is -1 rather than 0, because 0 is a size a caller could act
// on: a progress bar computed from a zero total divides by it.
func TestOpenReportsAnUnknownTotalAsNegative(t *testing.T) {
	server, client := server(t, func(w http.ResponseWriter, r *http.Request) {
		// Flushing before the body is complete is what makes the response carry no
		// `Content-Length`: the server has committed to an answer and does not yet
		// know how long it will be. Writing a small body and returning would let the
		// server compute the length and this test would assert nothing.
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		_, _ = w.Write(payload(64))
	})

	body, source, err := client.Open(context.Background(), server.URL, 0)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer body.Close()
	if source.TotalBytes != -1 {
		t.Fatalf("TotalBytes = %d, want -1 when no length was declared", source.TotalBytes)
	}
}

func TestOpenReportsANonSuccessStatusWithABoundedBody(t *testing.T) {
	server, client := server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(strings.Repeat("x", 4096)))
	})

	_, _, err := client.Open(context.Background(), server.URL+"/media/42.mp4?signature=DO-NOT-LOG-ME", 0)
	if err == nil {
		t.Fatal("Open() error = nil, want a refusal for a 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Fatalf("error = %v, want the status in it", err)
	}
	if len(err.Error()) > 1024 {
		t.Fatalf("error carries %d characters, want the body truncated to the prefix limit", len(err.Error()))
	}
}

// The address is a credential. An error that reaches a log or a task row must not
// carry it, and the transport's own wrapper is built out of the request URL.
func TestOpenKeepsTheSignedAddressOutOfEveryError(t *testing.T) {
	server, client := server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("denied"))
	})

	for _, testCase := range []struct {
		name    string
		address string
	}{
		{"a refused response", server.URL + "/media/42.mp4?signature=DO-NOT-LOG-ME"},
		{"an unroutable address", "http://127.0.0.1:1/media/42.mp4?signature=DO-NOT-LOG-ME"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, _, err := client.Open(context.Background(), testCase.address, 0)
			if err == nil {
				t.Fatal("Open() error = nil, want a refusal")
			}
			if strings.Contains(err.Error(), "DO-NOT-LOG-ME") {
				t.Fatalf("error carries the signed address: %v", err)
			}
		})
	}
}

// Refusing an address before the request is built is not politeness about error
// messages: `http.NewRequestWithContext` fails through `url.Parse`, whose own error
// text *is* the address. For a signed URL that puts the credential in the error —
// the exact thing this package keeps out of every other path.
func TestOpenRefusesAnAddressOrOffsetItCannotUse(t *testing.T) {
	server, client := server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("an invalid request must not reach the network")
	})

	for _, testCase := range []struct {
		name    string
		address string
		offset  int64
		want    string
	}{
		{"no address", "", 0, "required"},
		{"blank address", "   ", 0, "required"},
		{"a negative offset", server.URL, -1, "negative"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, _, err := client.Open(context.Background(), testCase.address, testCase.offset)
			if err == nil {
				t.Fatalf("Open(%q, %d) error = nil, want a refusal", testCase.address, testCase.offset)
			}
			if !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("error = %v, want the refusal to say %q rather than leaving it to the URL parser", err, testCase.want)
			}
		})
	}
}

// A cancellation has to stay recognisable as one through the wrapper: a download
// that was asked to stop is answered differently from one that failed.
func TestOpenKeepsACancellationRecognisable(t *testing.T) {
	server, client := server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload(16))
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := client.Open(ctx, server.URL, 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestSourceReportsTheContentType(t *testing.T) {
	server, client := server(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write(payload(8))
	})

	body, source, err := client.Open(context.Background(), server.URL, 0)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer body.Close()
	if source.ContentType != "video/mp4" {
		t.Fatalf("ContentType = %q", source.ContentType)
	}
}

// `http.ServeContent` is the standard library's own range implementation, so a
// second, larger file keeps the readings above from being an artefact of their
// size, and it pins the byte the body starts at rather than only its length.
func TestOpenAgainstAServedFileReportsItsWholeSize(t *testing.T) {
	size := 4096
	server, client := server(t, func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "42.mp4", zeroModTime(), strings.NewReader(string(payload(size))))
	})

	body, source, err := client.Open(context.Background(), server.URL, 1024)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer body.Close()
	if source.TotalBytes != int64(size) {
		t.Fatalf("TotalBytes = %d, want %d", source.TotalBytes, size)
	}
	head := make([]byte, 4)
	if _, err := io.ReadFull(body, head); err != nil {
		t.Fatalf("read head: %v", err)
	}
	if string(head) != string(payload(size)[1024:1028]) {
		t.Fatalf("the body does not start at the offset asked for: %q", string(head))
	}
}

func TestParseContentRange(t *testing.T) {
	for _, testCase := range []struct {
		value     string
		start     int64
		total     int64
		wantValid bool
	}{
		{"bytes 0-99/100", 0, 100, true},
		{"bytes 400-999/*", 400, -1, true},
		{" bytes 7-9/10 ", 7, 10, true},
		{"bytes 400-999", 0, 0, false},
		{"bytes -999/1000", 0, 0, false},
		{"items 0-9/10", 0, 0, false},
		// The unit is mandatory, and this is the case that says so: without the
		// check for it the numbers alone parse and a header naming some other unit
		// would be read as byte offsets.
		{"0-9/10", 0, 0, false},
		{"", 0, 0, false},
	} {
		t.Run(fmt.Sprintf("%q", testCase.value), func(t *testing.T) {
			start, total, ok := parseContentRange(testCase.value)
			if ok != testCase.wantValid {
				t.Fatalf("parseContentRange(%q) valid = %t, want %t", testCase.value, ok, testCase.wantValid)
			}
			if !ok {
				return
			}
			if start != testCase.start || total != testCase.total {
				t.Fatalf("parseContentRange(%q) = %d,%d want %d,%d", testCase.value, start, total, testCase.start, testCase.total)
			}
		})
	}
}

func TestNewWithClientRequiresAClient(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewWithClient(nil) must panic rather than fail on the first request")
		}
	}()
	NewWithClient(nil)
}
