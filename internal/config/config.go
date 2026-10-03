package config

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"jlu-drcom-win/internal/protocol"
)

type Config struct {
	Username string
	Password string
	IP       [4]byte
	MAC      [6]byte
	HostName string
	OSInfo   string

	ServerIP   [4]byte
	ServerPort int
	BindIP     [4]byte
	BindPort   int

	AuthVersion           [2]byte
	KeepAliveVersion      [2]byte
	FirstHeartbeatVersion [2]byte
	ExtraHeartbeatVersion [2]byte

	PrimaryDNS [4]byte
	DHCPServer [4]byte

	DebugHexDump          bool
	ReceiveTimeout        time.Duration
	RetryCount            int
	HeartbeatInterval     time.Duration
	ReceiveTimeoutMillis  int
	HeartbeatIntervalSecs int
	AdapterHint           string
	AutoNetwork           NetworkInfo
}

type NetworkInfo struct {
	InterfaceName string
	IP            [4]byte
	MAC           [6]byte
}

type NetworkDetector func(adapterHint string) (NetworkInfo, error)

var ErrNetworkUnavailable = errors.New("campus adapter is not ready")
var ErrAmbiguousNetwork = errors.New("multiple usable adapters/addresses; set adapter_hint or explicit ip and mac")

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	return Parse(string(data))
}

func Parse(data string) (Config, error) {
	return ParseWithDetector(data, AutoDetectNetwork)
}

func ParseWithDetector(data string, detect NetworkDetector) (Config, error) {
	values, err := parseTOML(data)
	if err != nil {
		return Config{}, err
	}
	if detect == nil {
		detect = AutoDetectNetwork
	}

	var cfg Config
	if cfg.Username, err = requiredString(values, "username"); err != nil {
		return cfg, err
	}
	if cfg.Password, err = requiredString(values, "password"); err != nil {
		return cfg, err
	}

	cfg.AdapterHint, err = optionalString(values, "adapter_hint", "")
	if err != nil {
		return cfg, err
	}

	needAutoNetwork := isAutoValue(values, "ip") || isAutoValue(values, "mac")
	if needAutoNetwork {
		cfg.AutoNetwork, err = detect(cfg.AdapterHint)
		if err != nil {
			if errors.Is(err, ErrAmbiguousNetwork) {
				return cfg, err
			}
			return cfg, fmt.Errorf("%w: %v", ErrNetworkUnavailable, err)
		}
	}

	if cfg.IP, err = optionalIPv4(values, "ip", cfg.AutoNetwork.IP); err != nil {
		return cfg, err
	}
	if cfg.MAC, err = optionalMAC(values, "mac", cfg.AutoNetwork.MAC); err != nil {
		return cfg, err
	}
	if needAutoNetwork && (cfg.IP != cfg.AutoNetwork.IP || cfg.MAC != cfg.AutoNetwork.MAC) {
		return cfg, fmt.Errorf("ip and mac must belong to the same adapter; use both auto with adapter_hint, or set both explicitly")
	}

	cfg.HostName, err = optionalString(values, "host_name", defaultHostName())
	if err != nil {
		return cfg, err
	}
	cfg.OSInfo, err = optionalString(values, "os_info", defaultOSInfo())
	if err != nil {
		return cfg, err
	}
	if cfg.ServerIP, err = optionalIPv4(values, "server_ip", [4]byte{10, 100, 61, 3}); err != nil {
		return cfg, err
	}
	if cfg.BindIP, err = optionalIPv4(values, "bind_ip", cfg.IP); err != nil {
		return cfg, err
	}
	if cfg.PrimaryDNS, err = optionalIPv4(values, "primary_dns", [4]byte{10, 10, 10, 10}); err != nil {
		return cfg, err
	}
	if cfg.DHCPServer, err = optionalIPv4(values, "dhcp_server", [4]byte{0, 0, 0, 0}); err != nil {
		return cfg, err
	}
	if cfg.AuthVersion, err = optionalHex2(values, "auth_version", [2]byte{0x68, 0x00}); err != nil {
		return cfg, err
	}
	if cfg.KeepAliveVersion, err = optionalHex2(values, "keepalive_version", [2]byte{0xdc, 0x02}); err != nil {
		return cfg, err
	}
	if cfg.FirstHeartbeatVersion, err = optionalHex2(values, "first_heartbeat_version", [2]byte{0x0f, 0x27}); err != nil {
		return cfg, err
	}
	if cfg.ExtraHeartbeatVersion, err = optionalHex2(values, "extra_heartbeat_version", [2]byte{0xdb, 0x02}); err != nil {
		return cfg, err
	}
	if cfg.ServerPort, err = optionalInt(values, "server_port", 61440); err != nil {
		return cfg, err
	}
	if cfg.BindPort, err = optionalInt(values, "bind_port", 61440); err != nil {
		return cfg, err
	}
	if cfg.ReceiveTimeoutMillis, err = optionalInt(values, "receive_timeout_ms", 2000); err != nil {
		return cfg, err
	}
	if cfg.RetryCount, err = optionalInt(values, "retry_count", 3); err != nil {
		return cfg, err
	}
	if cfg.HeartbeatIntervalSecs, err = optionalInt(values, "heartbeat_interval_seconds", 20); err != nil {
		return cfg, err
	}
	if cfg.DebugHexDump, err = optionalBool(values, "debug_hex_dump", false); err != nil {
		return cfg, err
	}

	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	cfg.ReceiveTimeout = time.Duration(cfg.ReceiveTimeoutMillis) * time.Millisecond
	cfg.HeartbeatInterval = time.Duration(cfg.HeartbeatIntervalSecs) * time.Second
	return cfg, nil
}

func AutoDetectNetwork(adapterHint string) (NetworkInfo, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return NetworkInfo{}, err
	}
	var candidates []networkCandidate
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip := ipv4FromAddr(addr)
			if ip == nil || !usableIPv4(ip) {
				continue
			}
			if len(iface.HardwareAddr) < 6 {
				continue
			}
			var mac [6]byte
			copy(mac[:], iface.HardwareAddr)
			candidates = append(candidates, networkCandidate{
				Name:  iface.Name,
				Flags: iface.Flags,
				IP:    [4]byte{ip[0], ip[1], ip[2], ip[3]},
				MAC:   mac,
			})
		}
	}
	return selectAutoNetwork(candidates, adapterHint)
}

type networkCandidate struct {
	Name  string
	Flags net.Flags
	IP    [4]byte
	MAC   [6]byte
}

func selectAutoNetwork(candidates []networkCandidate, adapterHint string) (NetworkInfo, error) {
	hint := strings.ToLower(strings.TrimSpace(adapterHint))
	var best networkCandidate
	bestScore := -1
	ambiguous := false
	for _, c := range candidates {
		if c.Flags&net.FlagUp == 0 || c.Flags&net.FlagLoopback != 0 {
			continue
		}
		if isZeroMAC(c.MAC) {
			continue
		}
		name := strings.ToLower(c.Name)
		if hint == "" && looksVirtualAdapter(name) {
			continue
		}
		score := 10
		if !looksVirtualAdapter(name) {
			score += 100
		}
		if hint != "" {
			if name == hint {
				score += 1000
			} else if strings.Contains(name, hint) {
				score += 500
			} else {
				continue
			}
		}
		if score > bestScore {
			best = c
			bestScore = score
			ambiguous = false
		} else if score == bestScore && (best.Name != c.Name || best.IP != c.IP) {
			ambiguous = true
		}
	}
	if bestScore < 0 {
		if hint != "" {
			return NetworkInfo{}, fmt.Errorf("auto network detection found no usable IPv4 adapter matching %q", adapterHint)
		}
		return NetworkInfo{}, fmt.Errorf("auto network detection found no usable IPv4 adapter")
	}
	if ambiguous {
		return NetworkInfo{}, ErrAmbiguousNetwork
	}
	return NetworkInfo{
		InterfaceName: best.Name,
		IP:            best.IP,
		MAC:           best.MAC,
	}, nil
}

func ipv4FromAddr(addr net.Addr) net.IP {
	switch v := addr.(type) {
	case *net.IPNet:
		return v.IP.To4()
	case *net.IPAddr:
		return v.IP.To4()
	default:
		return nil
	}
}

func usableIPv4(ip net.IP) bool {
	if ip == nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() {
		return false
	}
	return !(ip[0] == 169 && ip[1] == 254)
}

func looksVirtualAdapter(name string) bool {
	virtualWords := []string{
		"virtual",
		"vethernet",
		"hyper-v",
		"vmware",
		"virtualbox",
		"wsl",
		"docker",
		"bluetooth",
		"tailscale",
		"zerotier",
		"npcap",
		"loopback",
		"mihomo",
		"clash",
		"wintun",
		"wireguard",
	}
	for _, word := range virtualWords {
		if strings.Contains(name, word) {
			return true
		}
	}
	return false
}

func isZeroMAC(mac [6]byte) bool {
	return mac == [6]byte{}
}

func (c Config) Validate() error {
	if len([]byte(c.Username)) == 0 || len([]byte(c.Username)) > 36 {
		return fmt.Errorf("username length must be 1..36 bytes")
	}
	if len([]byte(c.Password)) == 0 || len([]byte(c.Password)) > 255 {
		return fmt.Errorf("password length must be 1..255 bytes")
	}
	if len([]byte(c.HostName)) > 32 {
		return fmt.Errorf("host_name length must be <= 32 bytes")
	}
	if len([]byte(c.OSInfo)) > 54 {
		return fmt.Errorf("os_info length must be <= 54 bytes")
	}
	if c.ServerPort <= 0 || c.ServerPort > 65535 {
		return fmt.Errorf("server_port must be 1..65535")
	}
	if c.BindPort <= 0 || c.BindPort > 65535 {
		return fmt.Errorf("bind_port must be 1..65535")
	}
	if c.ReceiveTimeoutMillis < 100 || c.ReceiveTimeoutMillis > 30000 {
		return fmt.Errorf("receive_timeout_ms must be 100..30000")
	}
	if c.RetryCount < 0 || c.RetryCount > 10 {
		return fmt.Errorf("retry_count must be 0..10")
	}
	if c.HeartbeatIntervalSecs < 1 || c.HeartbeatIntervalSecs > 300 {
		return fmt.Errorf("heartbeat_interval_seconds must be 1..300")
	}
	if !usableIPv4(ipv4ToNetIP(c.IP)) || isZeroMAC(c.MAC) {
		return fmt.Errorf("ip/mac must identify a usable adapter")
	}
	if c.BindIP != c.IP {
		return fmt.Errorf("bind_ip must equal the authentication ip (or auto)")
	}
	return nil
}

func (c Config) ProtocolConfig() protocol.Config {
	return protocol.Config{
		Username:              c.Username,
		Password:              c.Password,
		IP:                    c.IP,
		MAC:                   c.MAC,
		HostName:              c.HostName,
		OSInfo:                c.OSInfo,
		PrimaryDNS:            c.PrimaryDNS,
		DHCPServer:            c.DHCPServer,
		AuthVersion:           c.AuthVersion,
		KeepAliveVersion:      c.KeepAliveVersion,
		FirstHeartbeatVersion: c.FirstHeartbeatVersion,
		ExtraHeartbeatVersion: c.ExtraHeartbeatVersion,
	}
}

func (c Config) ServerUDPAddr() *net.UDPAddr {
	return &net.UDPAddr{
		IP:   ipv4ToNetIP(c.ServerIP),
		Port: c.ServerPort,
	}
}

func (c Config) BindUDPAddr() *net.UDPAddr {
	return &net.UDPAddr{
		IP:   ipv4ToNetIP(c.BindIP),
		Port: c.BindPort,
	}
}

func (c Config) ServerAddrString() string {
	return fmt.Sprintf("%s:%d", ipv4String(c.ServerIP), c.ServerPort)
}

func (c Config) BindAddrString() string {
	return fmt.Sprintf("%s:%d", ipv4String(c.BindIP), c.BindPort)
}

func ipv4ToNetIP(v [4]byte) net.IP {
	return net.IPv4(v[0], v[1], v[2], v[3]).To4()
}

func ipv4String(v [4]byte) string {
	return fmt.Sprintf("%d.%d.%d.%d", v[0], v[1], v[2], v[3])
}

func parseTOML(data string) (map[string]any, error) {
	values := make(map[string]any)
	if _, err := toml.Decode(strings.TrimPrefix(data, "\ufeff"), &values); err != nil {
		// Decoder diagnostics can quote the password source line.
		return nil, fmt.Errorf("invalid TOML syntax; check quoting, types and duplicate keys")
	}
	allowed := strings.Fields("username password ip mac adapter_hint host_name os_info server_ip server_port bind_ip bind_port primary_dns dhcp_server auth_version keepalive_version first_heartbeat_version extra_heartbeat_version debug_hex_dump receive_timeout_ms retry_count heartbeat_interval_seconds")
	for key := range values {
		known := false
		for _, k := range allowed {
			if key == k {
				known = true
				break
			}
		}
		if !known {
			return nil, fmt.Errorf("unknown configuration key (compare config.example.toml)")
		}
	}
	return values, nil
}

func requiredString(values map[string]any, key string) (string, error) {
	raw, ok := values[key]
	if !ok {
		return "", fmt.Errorf("missing required config key %q", key)
	}
	value, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a quoted string", key)
	}
	return value, nil
}

func optionalString(values map[string]any, key string, fallback string) (string, error) {
	if _, ok := values[key]; !ok {
		return fallback, nil
	}
	value, err := requiredString(values, key)
	if err != nil {
		return "", err
	}
	if strings.EqualFold(strings.TrimSpace(value), "auto") {
		return fallback, nil
	}
	return value, nil
}

func requiredInt(values map[string]any, key string) (int, error) {
	raw, ok := values[key]
	if !ok {
		return 0, fmt.Errorf("missing required config key %q", key)
	}
	n, ok := raw.(int64)
	if !ok || int64(int(n)) != n {
		return 0, fmt.Errorf("%s must be an integer in range", key)
	}
	return int(n), nil
}

func optionalInt(values map[string]any, key string, fallback int) (int, error) {
	if _, ok := values[key]; !ok {
		return fallback, nil
	}
	return requiredInt(values, key)
}

func requiredBool(values map[string]any, key string) (bool, error) {
	raw, ok := values[key]
	if !ok {
		return false, fmt.Errorf("missing required config key %q", key)
	}
	b, ok := raw.(bool)
	if !ok {
		return false, fmt.Errorf("%s must be true or false", key)
	}
	return b, nil
}

func optionalBool(values map[string]any, key string, fallback bool) (bool, error) {
	if _, ok := values[key]; !ok {
		return fallback, nil
	}
	return requiredBool(values, key)
}

func requiredIPv4(values map[string]any, key string) ([4]byte, error) {
	raw, err := requiredString(values, key)
	if err != nil {
		return [4]byte{}, err
	}
	ip := net.ParseIP(raw).To4()
	if ip == nil {
		return [4]byte{}, fmt.Errorf("%s must be an IPv4 address", key)
	}
	var out [4]byte
	copy(out[:], ip)
	return out, nil
}

func optionalIPv4(values map[string]any, key string, fallback [4]byte) ([4]byte, error) {
	if _, ok := values[key]; !ok {
		if fallback == [4]byte{} && key == "ip" {
			return fallback, fmt.Errorf("%s is auto but no usable adapter was detected", key)
		}
		return fallback, nil
	}
	raw, err := requiredString(values, key)
	if err != nil {
		return [4]byte{}, err
	}
	if strings.EqualFold(strings.TrimSpace(raw), "auto") {
		if fallback == [4]byte{} && key == "ip" {
			return fallback, fmt.Errorf("%s is auto but no usable adapter was detected", key)
		}
		return fallback, nil
	}
	ip := net.ParseIP(raw).To4()
	if ip == nil {
		return [4]byte{}, fmt.Errorf("%s must be an IPv4 address or auto", key)
	}
	var out [4]byte
	copy(out[:], ip)
	return out, nil
}

func requiredMAC(values map[string]any, key string) ([6]byte, error) {
	raw, err := requiredString(values, key)
	if err != nil {
		return [6]byte{}, err
	}
	return parseMAC(key, raw)
}

func optionalMAC(values map[string]any, key string, fallback [6]byte) ([6]byte, error) {
	if _, ok := values[key]; !ok {
		if fallback == [6]byte{} {
			return fallback, fmt.Errorf("%s is auto but no usable adapter was detected", key)
		}
		return fallback, nil
	}
	raw, err := requiredString(values, key)
	if err != nil {
		return [6]byte{}, err
	}
	if strings.EqualFold(strings.TrimSpace(raw), "auto") {
		if fallback == [6]byte{} {
			return fallback, fmt.Errorf("%s is auto but no usable adapter was detected", key)
		}
		return fallback, nil
	}
	return parseMAC(key, raw)
}

func parseMAC(key string, raw string) ([6]byte, error) {
	if !(strings.Count(raw, ":") == 5 || strings.Count(raw, "-") == 5) {
		return [6]byte{}, fmt.Errorf("%s must use 00:11:22:33:44:55, 00-11-22-33-44-55, or auto", key)
	}
	mac, err := net.ParseMAC(raw)
	if err != nil || len(mac) != 6 {
		return [6]byte{}, fmt.Errorf("%s must be a 6-byte MAC address", key)
	}
	var out [6]byte
	copy(out[:], mac)
	return out, nil
}

func requiredHex2(values map[string]any, key string) ([2]byte, error) {
	raw, err := requiredString(values, key)
	if err != nil {
		return [2]byte{}, err
	}
	if len(raw) != 4 {
		return [2]byte{}, fmt.Errorf("%s must be exactly 4 hex chars", key)
	}
	decoded, err := hex.DecodeString(raw)
	if err != nil {
		return [2]byte{}, fmt.Errorf("%s must be hex: %w", key, err)
	}
	var out [2]byte
	copy(out[:], decoded)
	return out, nil
}

func optionalHex2(values map[string]any, key string, fallback [2]byte) ([2]byte, error) {
	if _, ok := values[key]; !ok {
		return fallback, nil
	}
	return requiredHex2(values, key)
}

func isAutoValue(values map[string]any, key string) bool {
	_, ok := values[key]
	if !ok {
		return key == "ip" || key == "mac"
	}
	value, err := requiredString(values, key)
	if err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(value), "auto")
}

func defaultHostName() string {
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		return "windows-pc"
	}
	host = strings.TrimSpace(host)
	if len([]byte(host)) > 32 {
		return string([]byte(host)[:32])
	}
	return host
}

func defaultOSInfo() string {
	if runtime.GOOS == "windows" {
		return "Windows"
	}
	return runtime.GOOS
}
