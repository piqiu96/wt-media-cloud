package compose

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/production/executor"
)

const MaxGuideBytes int64 = 500 << 20

// DownloadGuide resolves and pins a public HTTPS endpoint for this one request.
// Proxies are disabled; each redirect is checked and each destination is
// resolved and pinned before a connection is opened.
func DownloadGuide(ctx context.Context, rawURL, path string) error {
	if err := executor.ValidateGuideURL(rawURL); err != nil {
		return err
	}
	transport := &http.Transport{
		Proxy:                 nil,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil || port != "443" {
				return nil, errors.New("guide endpoint must use HTTPS port 443")
			}
			ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
			if err != nil {
				return nil, fmt.Errorf("resolve guide host: %w", err)
			}
			if len(ips) == 0 {
				return nil, errors.New("guide host has no addresses")
			}
			for _, ip := range ips {
				if !publicGuideIP(ip) {
					return nil, errors.New("guide host resolves to a non-public address")
				}
			}
			return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
		},
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Minute, CheckRedirect: func(next *http.Request, via []*http.Request) error {
		if len(via) >= 3 || executor.ValidateGuideURL(next.URL.String()) != nil {
			return http.ErrUseLastResponse
		}
		return nil
	}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return errors.New("guide download failed; check that the address is reachable and has not expired")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download guide: HTTP %d", response.StatusCode)
	}
	return saveGuideBody(response.Body, response.ContentLength, path, MaxGuideBytes)
}

func saveGuideBody(body io.Reader, contentLength int64, path string, limit int64) error {
	if contentLength > limit {
		return errors.New("guide video exceeds 500 MiB")
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	n, err := io.Copy(file, io.LimitReader(body, limit+1))
	if err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("read guide video: %w", err)
	}
	if n == 0 || n > limit {
		_ = os.Remove(path)
		return errors.New("guide video is empty or exceeds 500 MiB")
	}
	return nil
}

func publicGuideIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	blocked := []netip.Prefix{
		netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"),
		netip.MustParsePrefix("169.254.0.0/16"), netip.MustParsePrefix("192.0.0.0/24"),
		netip.MustParsePrefix("192.0.2.0/24"), netip.MustParsePrefix("198.18.0.0/15"),
		netip.MustParsePrefix("198.51.100.0/24"), netip.MustParsePrefix("203.0.113.0/24"),
		netip.MustParsePrefix("224.0.0.0/4"), netip.MustParsePrefix("240.0.0.0/4"),
		netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("fc00::/7"),
		netip.MustParsePrefix("fe80::/10"), netip.MustParsePrefix("64:ff9b::/96"),
		netip.MustParsePrefix("2001::/32"), netip.MustParsePrefix("2002::/16"),
	}
	for _, prefix := range blocked {
		if prefix.Contains(ip) {
			return false
		}
	}
	return !strings.EqualFold(ip.String(), "::1")
}
