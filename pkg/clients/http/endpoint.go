package httpclient

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app/client/discovery"
	"github.com/cloudwego/hertz/pkg/app/client/loadbalance"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/network/dialer"
)

// EndpointConfig describes one logical HTTP endpoint and optional dial addresses.
type EndpointConfig struct {
	Scheme      string   `toml:"scheme"`
	Host        string   `toml:"host"`
	Port        int      `toml:"port"`
	Addresses   []string `toml:"addresses"`
	LoadBalance string   `toml:"load_balance"`
}

// Origin returns the logical URL origin used by semantic clients.
func (e EndpointConfig) Origin() string {
	return e.Scheme + "://" + e.LogicalAddress()
}

// LogicalAddress returns the host and port visible to HTTP and TLS.
func (e EndpointConfig) LogicalAddress() string {
	return net.JoinHostPort(e.Host, strconv.Itoa(e.Port))
}

func (e EndpointConfig) validate() error {
	if e.Scheme != "http" && e.Scheme != "https" {
		return errors.New("endpoint scheme must be http or https")
	}
	if !validEndpointHost(e.Host) {
		return errors.New("endpoint host must be an IP address or hostname without a port")
	}
	if e.Port < 1 || e.Port > 65535 {
		return errors.New("endpoint port must be between 1 and 65535")
	}
	if e.LoadBalance != "" && e.LoadBalance != "random" {
		return fmt.Errorf("invalid endpoint load_balance %q", e.LoadBalance)
	}
	if len(e.Addresses) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(e.Addresses))
	for _, address := range e.Addresses {
		host, portText, err := net.SplitHostPort(address)
		if err != nil || !validEndpointHost(host) {
			return fmt.Errorf("invalid endpoint address %q", address)
		}
		port, err := strconv.Atoi(portText)
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("invalid endpoint address port %q", address)
		}
		if _, duplicate := seen[address]; duplicate {
			return fmt.Errorf("duplicate endpoint address %q", address)
		}
		seen[address] = struct{}{}
	}
	return nil
}

func validEndpointHost(host string) bool {
	if host == "" || strings.ContainsAny(host, "/?#") {
		return false
	}
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		return net.ParseIP(host[1:len(host)-1]) != nil
	}
	return !strings.Contains(host, ":")
}

// endpointDialer keeps the logical request host intact while selecting one of
// the configured dial addresses. TLS SNI therefore continues to use Endpoint.Host.
type endpointDialer struct {
	inner          network.Dialer
	logicalAddress string
	instances      []discovery.Instance
	balancer       loadbalance.Loadbalancer
}

func newEndpointDialer(endpoint EndpointConfig) network.Dialer {
	addresses := endpoint.Addresses
	if len(addresses) == 0 {
		addresses = []string{endpoint.LogicalAddress()}
	}
	if len(addresses) == 1 && addresses[0] == endpoint.LogicalAddress() {
		return nil
	}

	instances := make([]discovery.Instance, 0, len(addresses))
	for _, address := range addresses {
		instances = append(instances, discovery.NewInstance("tcp", address, 1, nil))
	}
	return endpointDialer{
		inner:          dialer.DefaultDialer(),
		logicalAddress: endpoint.LogicalAddress(),
		instances:      instances,
		balancer:       loadbalance.NewWeightedBalancer(),
	}
}

func (d endpointDialer) DialConnection(network, address string, timeout time.Duration, tlsConfig *tls.Config) (network.Conn, error) {
	return d.inner.DialConnection(network, d.dialAddress(address), timeout, tlsConfig)
}

func (d endpointDialer) DialTimeout(network, address string, timeout time.Duration, tlsConfig *tls.Config) (net.Conn, error) {
	return d.inner.DialTimeout(network, d.dialAddress(address), timeout, tlsConfig)
}

func (d endpointDialer) AddTLS(conn network.Conn, tlsConfig *tls.Config) (network.Conn, error) {
	return d.inner.AddTLS(conn, tlsConfig)
}

func (d endpointDialer) dialAddress(address string) string {
	if address != d.logicalAddress {
		return address
	}
	instance := d.balancer.Pick(discovery.Result{CacheKey: d.logicalAddress, Instances: d.instances})
	if instance == nil {
		return address
	}
	return instance.Address().String()
}
