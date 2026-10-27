package service

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// parseProxyLine 接受运营实际粘贴的两种格式：
//
//	protocol://user:pass@host:port
//	host:port[:user[:pass]]
//
// IPv6 必须使用带方括号的 URL 形式；冒号形式有歧义，宁可拒绝也不猜测。
func parseProxyLine(raw string) (*CreateProxyInput, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("空行")
	}
	if strings.Contains(raw, "://") {
		return parseURLProxy(raw)
	}
	return parseColonProxy(raw)
}

func parseURLProxy(raw string) (*CreateProxyInput, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("地址格式无效: %s", raw)
	}
	protocol := ProxyProtocol(strings.ToLower(parsed.Scheme))
	switch protocol {
	case ProtocolHTTP, ProtocolHTTPS, ProtocolSOCKS5:
	default:
		return nil, fmt.Errorf("不支持的代理协议: %s", parsed.Scheme)
	}
	host := parsed.Hostname()
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port < 1 || port > 65535 {
		return nil, errors.New("代理端口无效")
	}
	if host == "" {
		return nil, errors.New("代理地址缺少主机")
	}
	input := &CreateProxyInput{ProxyProtocol: protocol, Host: host, Port: port}
	if parsed.User != nil {
		input.Username = parsed.User.Username()
		input.Password, _ = parsed.User.Password()
	}
	return input, nil
}

func parseColonProxy(raw string) (*CreateProxyInput, error) {
	parts := strings.SplitN(raw, ":", 4)
	if len(parts) < 2 {
		return nil, errors.New("需要 host:port 格式")
	}
	host := strings.TrimSpace(parts[0])
	if host == "" || strings.Contains(host, "[") {
		return nil, errors.New("代理地址无效；IPv6 请使用 protocol://host:port 格式")
	}
	port, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || port < 1 || port > 65535 {
		return nil, errors.New("代理端口无效")
	}
	input := &CreateProxyInput{ProxyProtocol: ProtocolHTTP, Host: host, Port: port}
	if len(parts) >= 3 {
		input.Username = parts[2]
	}
	if len(parts) >= 4 {
		input.Password = parts[3]
	}
	return input, nil
}
