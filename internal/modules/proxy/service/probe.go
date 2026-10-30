package service

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/model"
)

const (
	defaultProbeTarget  = "www.baidu.com:443"
	defaultProbeTimeout = 8 * time.Second
	maxProxyResponse    = 4 << 10
)

// ProbeOptions 界定连通性检测的可测范围。目标用真实外部主机，因此 ok 表示
// 代理确实转发了流量，而不只是 TCP 端口能连上。
type ProbeOptions struct {
	Target  string
	Timeout time.Duration
}

func (o ProbeOptions) normalized() ProbeOptions {
	if strings.TrimSpace(o.Target) == "" {
		o.Target = defaultProbeTarget
	}
	if o.Timeout <= 0 {
		o.Timeout = defaultProbeTimeout
	}
	return o
}

// Probe 在 Cloud 侧执行检测，结果不依赖运营当前用哪台工作站，也不要求比特
// 窗口已打开。它走 HTTP CONNECT 或 SOCKS5，并按代理的实际应答分类。
func Probe(protocol model.Protocol, host string, port int, username, password string, opts ProbeOptions) dto.ProxyCheckResult {
	opts = opts.normalized()
	host = strings.TrimSpace(host)
	if host == "" || port < 1 || port > 65535 {
		return dto.ProxyCheckResult{Connectivity: CheckInvalid, Reason: "代理地址或端口无效"}
	}
	switch protocol {
	case ProtocolHTTP, ProtocolHTTPS, ProtocolSOCKS5:
	default:
		return dto.ProxyCheckResult{Connectivity: CheckInvalid, Reason: "不支持的代理协议"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	conn, err := dialProxy(ctx, protocol, host, port, opts.Timeout)
	if err != nil {
		return classifyDialError(err, host, port)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(opts.Timeout))

	if protocol == ProtocolSOCKS5 {
		return probeSOCKS5(conn, opts.Target, username, password)
	}
	return probeHTTPConnect(conn, opts.Target, username, password)
}

func dialProxy(ctx context.Context, protocol model.Protocol, host string, port int, timeout time.Duration) (net.Conn, error) {
	address := proxyDialAddress(host, port)
	dialer := &net.Dialer{Timeout: timeout}
	if protocol != ProtocolHTTPS {
		return dialer.DialContext(ctx, "tcp", address)
	}
	// https 表示代理端点本身使用 TLS。
	conn, err := tls.DialWithDialer(dialer, "tcp", address, &tls.Config{ServerName: host})
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func classifyDialError(err error, host string, port int) dto.ProxyCheckResult {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return dto.ProxyCheckResult{Connectivity: CheckTimeout, Reason: "连接超时"}
	}
	return dto.ProxyCheckResult{
		Connectivity: CheckConnFailed,
		Reason:       fmt.Sprintf("无法连接 %s: %s", proxyDialAddress(host, port), dialErrorText(err)),
	}
}

// dialErrorText 让面向运营的失败原因保持简短且不含凭据。
func dialErrorText(err error) string {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "连接超时"
	}
	message := err.Error()
	if len([]rune(message)) > 120 {
		return string([]rune(message)[:120])
	}
	return message
}

func probeHTTPConnect(conn net.Conn, target, username, password string) dto.ProxyCheckResult {
	var request strings.Builder
	request.WriteString("CONNECT " + target + " HTTP/1.1\r\n")
	request.WriteString("Host: " + target + "\r\n")
	request.WriteString("Proxy-Connection: keep-alive\r\n")
	if username != "" {
		credential := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
		request.WriteString("Proxy-Authorization: Basic " + credential + "\r\n")
	}
	request.WriteString("\r\n")
	if _, err := io.WriteString(conn, request.String()); err != nil {
		return dto.ProxyCheckResult{Connectivity: CheckConnFailed, Reason: "写入代理请求失败: " + dialErrorText(err)}
	}

	reader := bufio.NewReader(io.LimitReader(conn, maxProxyResponse))
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		return dto.ProxyCheckResult{Connectivity: CheckConnFailed, Reason: "代理无响应: " + dialErrorText(err)}
	}
	code, ok := parseHTTPStatusCode(statusLine)
	if !ok {
		return dto.ProxyCheckResult{Connectivity: CheckConnFailed, Reason: "代理返回了无法识别的响应"}
	}
	switch {
	case code >= 200 && code < 300:
		return dto.ProxyCheckResult{Connectivity: CheckOK}
	case code == 407:
		return dto.ProxyCheckResult{Connectivity: CheckAuthFailed, Reason: "代理拒绝了用户名或密码"}
	case code == 401 || code == 403:
		return dto.ProxyCheckResult{Connectivity: CheckAuthFailed, Reason: fmt.Sprintf("代理返回 HTTP %d", code)}
	default:
		return dto.ProxyCheckResult{Connectivity: CheckConnFailed, Reason: fmt.Sprintf("代理返回 HTTP %d", code)}
	}
}

func parseHTTPStatusCode(statusLine string) (int, bool) {
	fields := strings.Fields(strings.TrimSpace(statusLine))
	if len(fields) < 2 || !strings.HasPrefix(fields[0], "HTTP/") {
		return 0, false
	}
	code, err := strconv.Atoi(fields[1])
	if err != nil || code < 100 || code > 599 {
		return 0, false
	}
	return code, true
}

func probeSOCKS5(conn net.Conn, target, username, password string) dto.ProxyCheckResult {
	host, portText, err := net.SplitHostPort(target)
	if err != nil {
		return dto.ProxyCheckResult{Connectivity: CheckInvalid, Reason: "探测目标无效"}
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return dto.ProxyCheckResult{Connectivity: CheckInvalid, Reason: "探测目标端口无效"}
	}

	methods := []byte{0x00}
	if username != "" {
		methods = []byte{0x00, 0x02}
	}
	greeting := append([]byte{0x05, byte(len(methods))}, methods...)
	if _, err := conn.Write(greeting); err != nil {
		return dto.ProxyCheckResult{Connectivity: CheckConnFailed, Reason: "写入代理握手失败: " + dialErrorText(err)}
	}
	reply := make([]byte, 2)
	if _, err := io.ReadFull(conn, reply); err != nil {
		return dto.ProxyCheckResult{Connectivity: CheckConnFailed, Reason: "代理无响应: " + dialErrorText(err)}
	}
	if reply[0] != 0x05 {
		return dto.ProxyCheckResult{Connectivity: CheckConnFailed, Reason: "代理未返回 SOCKS5 响应"}
	}
	switch reply[1] {
	case 0x00:
	case 0x02:
		if username == "" {
			return dto.ProxyCheckResult{Connectivity: CheckAuthFailed, Reason: "代理要求用户名和密码"}
		}
		if failure := socks5Authenticate(conn, username, password); failure != "" {
			return dto.ProxyCheckResult{Connectivity: CheckAuthFailed, Reason: failure}
		}
	case 0xFF:
		return dto.ProxyCheckResult{Connectivity: CheckAuthFailed, Reason: "代理不接受提供的认证方式"}
	default:
		return dto.ProxyCheckResult{Connectivity: CheckConnFailed, Reason: "代理要求不支持的认证方式"}
	}

	request := []byte{0x05, 0x01, 0x00}
	if ip := net.ParseIP(host); ip != nil {
		if ipv4 := ip.To4(); ipv4 != nil {
			request = append(request, 0x01)
			request = append(request, ipv4...)
		} else {
			request = append(request, 0x04)
			request = append(request, ip.To16()...)
		}
	} else {
		if len(host) > 255 {
			return dto.ProxyCheckResult{Connectivity: CheckInvalid, Reason: "探测目标域名过长"}
		}
		request = append(request, 0x03, byte(len(host)))
		request = append(request, host...)
	}
	request = binary.BigEndian.AppendUint16(request, uint16(port))
	if _, err := conn.Write(request); err != nil {
		return dto.ProxyCheckResult{Connectivity: CheckConnFailed, Reason: "写入代理请求失败: " + dialErrorText(err)}
	}

	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return dto.ProxyCheckResult{Connectivity: CheckConnFailed, Reason: "代理无响应: " + dialErrorText(err)}
	}
	if header[0] != 0x05 {
		return dto.ProxyCheckResult{Connectivity: CheckConnFailed, Reason: "代理返回了无法识别的响应"}
	}
	if header[1] != 0x00 {
		return dto.ProxyCheckResult{Connectivity: CheckConnFailed, Reason: socks5ReplyText(header[1])}
	}
	return dto.ProxyCheckResult{Connectivity: CheckOK}
}

func socks5Authenticate(conn net.Conn, username, password string) string {
	request := []byte{0x01}
	request = append(request, byte(len(username)))
	request = append(request, username...)
	request = append(request, byte(len(password)))
	request = append(request, password...)
	if _, err := conn.Write(request); err != nil {
		return "写入认证信息失败: " + dialErrorText(err)
	}
	reply := make([]byte, 2)
	if _, err := io.ReadFull(conn, reply); err != nil {
		return "代理无响应: " + dialErrorText(err)
	}
	if reply[1] != 0x00 {
		return "代理拒绝了用户名或密码"
	}
	return ""
}

func socks5ReplyText(code byte) string {
	switch code {
	case 0x01:
		return "代理内部错误"
	case 0x02:
		return "代理规则拒绝连接"
	case 0x03:
		return "代理网络不可达"
	case 0x04:
		return "探测目标不可达"
	case 0x05:
		return "代理连接探测目标被拒绝"
	case 0x06:
		return "代理连接探测目标超时"
	case 0x07:
		return "代理不支持该请求"
	case 0x08:
		return "代理不支持该地址类型"
	default:
		return fmt.Sprintf("代理返回错误码 %d", code)
	}
}
