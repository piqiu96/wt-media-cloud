// Package sourcefile streams a material's source video from the address its
// platform handed out.
//
// This is the one client in the tree that does not go through
// `pkg/clients/http`: that package binds a named transport to one configured
// origin, and a platform CDN address is an arbitrary origin that changes with
// every lookup. `internal/architecture/boundary_test.go` keeps `http.Client` out
// of `internal/modules/` so that this decision has to be made here, in
// `internal/infra/`, once.
//
// The addresses are short-lived signed URLs. They are never logged, never
// returned in an error message and never persisted: a caller gets the body and
// the facts about it, and the address it passed in stays in its own scope.
package sourcefile

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// Source describes the body a caller just opened.
type Source struct {
	// RangeStart is the offset the body's first byte corresponds to. A caller
	// resuming at an offset may append to the bytes it already has only when this
	// equals the offset it asked for: a server that ignores `Range` answers 200
	// with the whole file from zero, and appending that to a partial file would
	// produce a file of the right length and the wrong contents. Reporting the
	// offset rather than a boolean is what lets the caller tell "no resume" apart
	// from "resumed from somewhere else".
	RangeStart int64

	// TotalBytes is the whole object's size: from `Content-Range` when the server
	// answered a range, from `Content-Length` otherwise, and -1 when the server
	// said neither (a chunked response). An unknown total is not an error — the
	// download learns the size from what it receives — but it is not zero either,
	// which is why the unknown value is negative rather than the zero value.
	TotalBytes int64

	ContentType string
}

// Client opens source files. It owns its HTTP client because nothing about the
// addresses it is given is known in advance, including how long a request may
// take.
type Client struct {
	http *http.Client
}

// New builds a client with limits suited to a large file fetch: redirects are
// followed (platform CDNs do redirect), and no whole-request timeout is set
// because a legitimate download of a long video is measured in minutes. Stalls
// are the caller's to detect: it has the byte counts and the clock.
func New() *Client {
	return NewWithClient(&http.Client{Timeout: 0})
}

// NewWithClient builds a client over an HTTP client the caller owns, which is how
// a test reaches a local server.
func NewWithClient(client *http.Client) *Client {
	if client == nil {
		panic("sourcefile: HTTP client is required")
	}
	return &Client{http: client}
}

// Open requests the source file, starting at offset, and returns the body
// unread. The caller owns closing it.
//
// offset 0 asks for the whole file and sends no `Range` header, so a CDN that
// would rather stream from the start is not asked to do anything else. A positive
// offset asks to resume and the answer says whether it did.
//
// A non-2xx answer is an error carrying the status and a bounded prefix of the
// body. Neither the status text nor that prefix is trusted, and the address is
// deliberately absent from the message: it is a signed URL, and an error that
// travels into a log or a task row would carry the credential with it.
func (c *Client) Open(ctx context.Context, address string, offset int64) (io.ReadCloser, Source, error) {
	if strings.TrimSpace(address) == "" {
		return nil, Source{}, errors.New("source address is required")
	}
	if offset < 0 {
		return nil, Source{}, fmt.Errorf("source offset %d is negative", offset)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, Source{}, fmt.Errorf("build source request: %w", err)
	}
	if offset > 0 {
		request.Header.Set("Range", "bytes="+strconv.FormatInt(offset, 10)+"-")
	}
	response, err := c.http.Do(request)
	if err != nil {
		// The wrapped error is `*url.Error`, whose text contains the address. It is
		// unwrapped to its cause so the signed URL cannot reach a log through an
		// error chain nobody looked at.
		return nil, Source{}, fmt.Errorf("request source file: %w", transportCause(err))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		detail := readPrefix(response.Body)
		_ = response.Body.Close()
		return nil, Source{}, fmt.Errorf("source http status %d: %s", response.StatusCode, detail)
	}
	return response.Body, sourceOf(response, offset), nil
}

// sourceOf reads the facts out of the response headers.
//
// `Content-Range` is believed over the status code because it is the header that
// names the window: a 206 whose range starts somewhere other than the offset
// asked for would be silently wrong if the status alone were trusted.
func sourceOf(response *http.Response, offset int64) Source {
	source := Source{RangeStart: 0, TotalBytes: -1, ContentType: response.Header.Get("Content-Type")}
	if response.StatusCode == http.StatusPartialContent {
		start, total, ok := parseContentRange(response.Header.Get("Content-Range"))
		if ok {
			source.RangeStart, source.TotalBytes = start, total
			return source
		}
		// A 206 with no usable `Content-Range` is a server saying "partial" without
		// saying which part. The only offset it can be trusted to mean is the one
		// that was asked for, and the total stays unknown rather than being guessed
		// from a `Content-Length` that describes this fragment.
		source.RangeStart = offset
		return source
	}
	if length, err := strconv.ParseInt(response.Header.Get("Content-Length"), 10, 64); err == nil && length >= 0 {
		source.TotalBytes = length
	}
	return source
}

// parseContentRange reads `bytes <start>-<end>/<total>`, where total may be `*`.
func parseContentRange(value string) (int64, int64, bool) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "bytes ") {
		return 0, 0, false
	}
	span, total, ok := strings.Cut(strings.TrimSpace(strings.TrimPrefix(value, "bytes ")), "/")
	if !ok {
		return 0, 0, false
	}
	start, _, ok := strings.Cut(span, "-")
	if !ok {
		return 0, 0, false
	}
	startValue, err := strconv.ParseInt(strings.TrimSpace(start), 10, 64)
	if err != nil || startValue < 0 {
		return 0, 0, false
	}
	if strings.TrimSpace(total) == "*" {
		return startValue, -1, true
	}
	totalValue, err := strconv.ParseInt(strings.TrimSpace(total), 10, 64)
	if err != nil || totalValue < 0 {
		return 0, 0, false
	}
	return startValue, totalValue, true
}

// transportCause strips the address out of a transport failure.
//
// `url.Error` is the transport's own wrapper and its `Error()` is the request URL
// plus the cause, so the cause alone is what may travel. A context error is
// returned as-is: a cancelled download has to stay recognisable as one, because a
// cancellation and a network fault are answered differently.
func transportCause(err error) error {
	var urlErr interface{ Unwrap() error }
	if errors.As(err, &urlErr) {
		if cause := urlErr.Unwrap(); cause != nil {
			return cause
		}
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return errors.New("source transport failure")
}

// readPrefix is how much of an error body is kept. A CDN's error page can be an
// entire HTML document and its first line is the part that says what happened.
const errorPrefixLimit = 512

func readPrefix(body io.Reader) string {
	prefix, err := io.ReadAll(io.LimitReader(body, errorPrefixLimit))
	if err != nil {
		return "(unreadable body)"
	}
	return strings.TrimSpace(string(prefix))
}

// Default is the client for callers that have no reason to build their own.
var defaultClient = New()

// Open fetches with the package's default client.
func Open(ctx context.Context, address string, offset int64) (io.ReadCloser, Source, error) {
	return defaultClient.Open(ctx, address, offset)
}
