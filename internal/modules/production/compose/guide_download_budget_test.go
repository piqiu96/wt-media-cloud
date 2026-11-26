//go:build go1.25

package compose

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"
)

type guideRoundTrip func(*http.Request) (*http.Response, error)

func (f guideRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type guideDripBody struct {
	ctx       context.Context
	remaining int
}

func (r *guideDripBody) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	case <-time.After(time.Second):
		p[0] = 'v'
		r.remaining--
		return 1, nil
	}
}
func (*guideDripBody) Close() error { return nil }
func slowGuideClient() *http.Client {
	return guideHTTPClient(guideRoundTrip(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: &guideDripBody{ctx: r.Context(), remaining: 480}, ContentLength: 480, Request: r}, nil
	}))
}
func TestHealthyGuideTransferCanFinishAfterFiveMinutes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "guide.mp4")
	synctest.Test(t, func(t *testing.T) {
		response, err := slowGuideClient().Get("https://guide.example/video.mp4")
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if err = saveGuideBody(response.Body, response.ContentLength, path, MaxGuideBytes); err != nil {
			t.Fatalf("continuous eight-minute transfer: %v", err)
		}
		data, err := os.ReadFile(path)
		if err != nil || len(data) != 480 {
			t.Fatalf("downloaded bytes=%d error=%v", len(data), err)
		}
	})
}
func TestGuideTransferStillHonorsEarlierTaskCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "guide.mp4")
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		request, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://guide.example/video.mp4", nil)
		response, err := slowGuideClient().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if err = saveGuideBody(response.Body, response.ContentLength, path, MaxGuideBytes); err == nil {
			t.Fatal("cancelled transfer committed")
		}
		if _, err = os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("partial resource remains: %v", err)
		}
	})
}

func TestGuideReadIdleTimeoutDoesNotLimitTotalProgress(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		receiver, sender := net.Pipe()
		defer receiver.Close()
		defer sender.Close()
		conn := &guideReadIdleConn{Conn: receiver}
		go func() {
			time.Sleep(20 * time.Second)
			sender.Write([]byte("a"))
			time.Sleep(20 * time.Second)
			sender.Write([]byte("b"))
		}()
		var p [1]byte
		for _, want := range []byte("ab") {
			if n, err := conn.Read(p[:]); err != nil || n != 1 || p[0] != want {
				t.Fatalf("progressing read: n=%d byte=%q err=%v", n, p[0], err)
			}
		}
	})
}
func TestGuideReadIdleTimeoutStopsAStalledConnection(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		receiver, sender := net.Pipe()
		defer receiver.Close()
		defer sender.Close()
		conn := &guideReadIdleConn{Conn: receiver}
		started := time.Now()
		var p [1]byte
		_, err := conn.Read(p[:])
		timeout, ok := err.(net.Error)
		if !ok || !timeout.Timeout() || time.Since(started) != 30*time.Second {
			t.Fatalf("stalled read: elapsed=%v err=%v", time.Since(started), err)
		}
	})
}
