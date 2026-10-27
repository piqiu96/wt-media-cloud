package service

import (
	"bufio"
	"encoding/base64"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestProbeHTTPSpeaksConnectAndClassifiesTheAnswer(t *testing.T) {
	server := startProxyStub(t, func(conn net.Conn) {
		reader := bufio.NewReader(conn)
		request, _ := reader.ReadString('\n')
		for {
			line, err := reader.ReadString('\n')
			if err != nil || strings.TrimSpace(line) == "" {
				break
			}
		}
		switch {
		case strings.HasPrefix(request, "CONNECT"):
			_, _ = io.WriteString(conn, "HTTP/1.1 200 Connection Established\r\n\r\n")
		}
	})
	if result := Probe(ProtocolHTTP, "127.0.0.1", server.port, "", "", fastProbeOptions()); result.Connectivity != CheckOK {
		t.Fatalf("result = %#v", result)
	}
}

func TestProbeHTTPReportsAuthenticationFailure(t *testing.T) {
	server := startProxyStub(t, func(conn net.Conn) {
		reader := bufio.NewReader(conn)
		_, _ = reader.ReadString('\n')
		for {
			line, err := reader.ReadString('\n')
			if err != nil || strings.TrimSpace(line) == "" {
				break
			}
		}
		_, _ = io.WriteString(conn, "HTTP/1.1 407 Proxy Authentication Required\r\n\r\n")
	})
	result := Probe(ProtocolHTTP, "127.0.0.1", server.port, "alice", "wrong", fastProbeOptions())
	if result.Connectivity != CheckAuthFailed {
		t.Fatalf("result = %#v, want auth_failed", result)
	}
}

func TestProbeHTTPPresentsBasicCredentials(t *testing.T) {
	seen := make(chan string, 1)
	server := startProxyStub(t, func(conn net.Conn) {
		reader := bufio.NewReader(conn)
		_, _ = reader.ReadString('\n')
		authorization := ""
		for {
			line, err := reader.ReadString('\n')
			if err != nil || strings.TrimSpace(line) == "" {
				break
			}
			if strings.HasPrefix(strings.ToLower(line), "proxy-authorization:") {
				authorization = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			}
		}
		seen <- authorization
		_, _ = io.WriteString(conn, "HTTP/1.1 200 Connection Established\r\n\r\n")
	})
	if result := Probe(ProtocolHTTP, "127.0.0.1", server.port, "alice", "secret", fastProbeOptions()); result.Connectivity != CheckOK {
		t.Fatalf("result = %#v", result)
	}
	expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("alice:secret"))
	if got := <-seen; got != expected {
		t.Fatalf("Proxy-Authorization = %q, want %q", got, expected)
	}
}

func TestProbeSOCKS5WithoutCredentialsFailsWhenAuthenticationIsRequired(t *testing.T) {
	server := startProxyStub(t, func(conn net.Conn) {
		greeting := make([]byte, 2)
		if _, err := io.ReadFull(conn, greeting); err != nil {
			return
		}
		methods := make([]byte, greeting[1])
		_, _ = io.ReadFull(conn, methods)
		_, _ = conn.Write([]byte{0x05, 0x02})
	})
	result := Probe(ProtocolSOCKS5, "127.0.0.1", server.port, "", "", fastProbeOptions())
	if result.Connectivity != CheckAuthFailed {
		t.Fatalf("result = %#v, want auth_failed", result)
	}
}

func TestProbeSOCKS5CompletesHandshakeAfterAuthentication(t *testing.T) {
	server := startProxyStub(t, func(conn net.Conn) {
		greeting := make([]byte, 2)
		if _, err := io.ReadFull(conn, greeting); err != nil {
			return
		}
		methods := make([]byte, greeting[1])
		_, _ = io.ReadFull(conn, methods)
		_, _ = conn.Write([]byte{0x05, 0x02})
		header := make([]byte, 2)
		if _, err := io.ReadFull(conn, header); err != nil {
			return
		}
		user := make([]byte, header[1])
		_, _ = io.ReadFull(conn, user)
		passwordLength := make([]byte, 1)
		_, _ = io.ReadFull(conn, passwordLength)
		password := make([]byte, passwordLength[0])
		_, _ = io.ReadFull(conn, password)
		if string(user) != "alice" || string(password) != "secret" {
			_, _ = conn.Write([]byte{0x01, 0x01})
			return
		}
		_, _ = conn.Write([]byte{0x01, 0x00})
		request := make([]byte, 4)
		if _, err := io.ReadFull(conn, request); err != nil {
			return
		}
		addressLength := int(request[3])
		if request[3] == 0x03 {
			address := make([]byte, addressLength)
			_, _ = io.ReadFull(conn, address)
			_, _ = io.ReadFull(conn, make([]byte, 2))
		}
		_, _ = conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
	})
	result := Probe(ProtocolSOCKS5, "127.0.0.1", server.port, "alice", "secret", fastProbeOptions())
	if result.Connectivity != CheckOK {
		t.Fatalf("result = %#v, want ok", result)
	}
}

func TestProbeSOCKS5ReportsRejectedCredentials(t *testing.T) {
	server := startProxyStub(t, func(conn net.Conn) {
		greeting := make([]byte, 2)
		if _, err := io.ReadFull(conn, greeting); err != nil {
			return
		}
		_, _ = io.ReadFull(conn, make([]byte, greeting[1]))
		_, _ = conn.Write([]byte{0x05, 0x02})
		header := make([]byte, 2)
		if _, err := io.ReadFull(conn, header); err != nil {
			return
		}
		_, _ = io.ReadFull(conn, make([]byte, int(header[1])))
		passwordLength := make([]byte, 1)
		_, _ = io.ReadFull(conn, passwordLength)
		_, _ = io.ReadFull(conn, make([]byte, int(passwordLength[0])))
		_, _ = conn.Write([]byte{0x01, 0x01})
	})
	result := Probe(ProtocolSOCKS5, "127.0.0.1", server.port, "alice", "wrong", fastProbeOptions())
	if result.Connectivity != CheckAuthFailed {
		t.Fatalf("result = %#v, want auth_failed", result)
	}
}

func TestProbeClassifiesConnectionAndParameterFailures(t *testing.T) {
	closed := startProxyStub(t, func(net.Conn) {})
	closed.close()
	if result := Probe(ProtocolSOCKS5, "127.0.0.1", closed.port, "", "", fastProbeOptions()); result.Connectivity != CheckConnFailed {
		t.Fatalf("closed port result = %#v, want conn_failed", result)
	}
	if result := Probe(ProtocolHTTP, "127.0.0.1", 70000, "", "", fastProbeOptions()); result.Connectivity != CheckInvalid {
		t.Fatalf("invalid port result = %#v, want invalid_params", result)
	}
	if result := Probe("ftp", "127.0.0.1", 21, "", "", fastProbeOptions()); result.Connectivity != CheckInvalid {
		t.Fatalf("unsupported protocol result = %#v, want invalid_params", result)
	}
}

// --- 测试替身 ---

type proxyStub struct {
	port  int
	done  chan struct{}
	close func()
}

func startProxyStub(t *testing.T, handle func(net.Conn)) *proxyStub {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	stub := &proxyStub{port: listener.Addr().(*net.TCPAddr).Port, done: make(chan struct{})}
	var once sync.Once
	stub.close = func() {
		once.Do(func() {
			_ = listener.Close()
			<-stub.done
		})
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				close(stub.done)
				return
			}
			handle(conn)
			_ = conn.Close()
		}
	}()
	t.Cleanup(stub.close)
	return stub
}

func fastProbeOptions() ProbeOptions {
	return ProbeOptions{Target: "example.com:443", Timeout: 2 * time.Second}
}
