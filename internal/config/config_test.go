package config

import (
	"net"
	"testing"
)

func TestParseConfigPreservesZeroByteHexVersion(t *testing.T) {
	cfg, err := Parse(validConfig())
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if cfg.AuthVersion != [2]byte{0x68, 0x00} {
		t.Fatalf("auth version = % x", cfg.AuthVersion)
	}
	if cfg.KeepAliveVersion != [2]byte{0xdc, 0x02} {
		t.Fatalf("keepalive version = % x", cfg.KeepAliveVersion)
	}
	if cfg.MAC != [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55} {
		t.Fatalf("mac = % x", cfg.MAC)
	}
	if cfg.ReceiveTimeoutMillis != 2000 || cfg.ReceiveTimeout == 0 {
		t.Fatalf("receive timeout not parsed")
	}
}

func TestParseConfigAcceptsHyphenMAC(t *testing.T) {
	data := replaceLine(validConfig(), `mac = "00:11:22:33:44:55"`, `mac = "00-11-22-33-44-55"`)
	cfg, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if cfg.MAC != [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55} {
		t.Fatalf("mac = % x", cfg.MAC)
	}
}

func TestParseConfigRejectsLongVersion(t *testing.T) {
	data := replaceLine(validConfig(), `auth_version = "6800"`, `auth_version = "680000"`)
	if _, err := Parse(data); err == nil {
		t.Fatalf("Parse() expected error for long auth_version")
	}
}

func TestParseConfigRejectsLongHostName(t *testing.T) {
	data := replaceLine(validConfig(), `host_name = "windows-pc"`, `host_name = "abcdefghijklmnopqrstuvwxyz1234567"`)
	if _, err := Parse(data); err == nil {
		t.Fatalf("Parse() expected error for long host_name")
	}
}

func TestParseMinimalConfigAutoDetectsNetworkAndAppliesDefaults(t *testing.T) {
	cfg, err := ParseWithDetector(`
username = "student_id"
password = "password"
`, func(adapterHint string) (NetworkInfo, error) {
		if adapterHint != "" {
			t.Fatalf("adapterHint = %q", adapterHint)
		}
		return NetworkInfo{
			InterfaceName: "Ethernet",
			IP:            [4]byte{49, 140, 167, 234},
			MAC:           [6]byte{0x74, 0xd4, 0xdd, 0xd1, 0xdc, 0x37},
		}, nil
	})
	if err != nil {
		t.Fatalf("ParseWithDetector() error = %v", err)
	}
	if cfg.IP != [4]byte{49, 140, 167, 234} {
		t.Fatalf("auto ip = %v", cfg.IP)
	}
	if cfg.MAC != [6]byte{0x74, 0xd4, 0xdd, 0xd1, 0xdc, 0x37} {
		t.Fatalf("auto mac = % x", cfg.MAC)
	}
	if cfg.ServerIP != [4]byte{10, 100, 61, 3} || cfg.ServerPort != 61440 || cfg.BindPort != 61440 {
		t.Fatalf("defaults not applied: server=%v port=%d bind=%d", cfg.ServerIP, cfg.ServerPort, cfg.BindPort)
	}
	if cfg.AuthVersion != [2]byte{0x68, 0x00} || cfg.KeepAliveVersion != [2]byte{0xdc, 0x02} {
		t.Fatalf("protocol defaults not applied")
	}
}

func TestParseConfigPassesAdapterHintToAutoDetect(t *testing.T) {
	cfg, err := ParseWithDetector(`
username = "student_id"
password = "password"
adapter_hint = "ethernet"
ip = "auto"
mac = "auto"
`, func(adapterHint string) (NetworkInfo, error) {
		if adapterHint != "ethernet" {
			t.Fatalf("adapterHint = %q", adapterHint)
		}
		return NetworkInfo{
			InterfaceName: "Ethernet",
			IP:            [4]byte{10, 1, 2, 3},
			MAC:           [6]byte{0, 1, 2, 3, 4, 5},
		}, nil
	})
	if err != nil {
		t.Fatalf("ParseWithDetector() error = %v", err)
	}
	if cfg.AdapterHint != "ethernet" || cfg.AutoNetwork.InterfaceName != "Ethernet" {
		t.Fatalf("adapter fields = %q / %q", cfg.AdapterHint, cfg.AutoNetwork.InterfaceName)
	}
}

func TestSelectAutoNetworkPrefersPhysicalAdapter(t *testing.T) {
	got, err := selectAutoNetwork([]networkCandidate{
		{
			Name:  "vEthernet (Default Switch)",
			Flags: net.FlagUp,
			IP:    [4]byte{172, 21, 48, 1},
			MAC:   [6]byte{0, 0x15, 0x5d, 0xbd, 0x69, 0x04},
		},
		{
			Name:  "以太网",
			Flags: net.FlagUp,
			IP:    [4]byte{49, 140, 167, 234},
			MAC:   [6]byte{0x74, 0xd4, 0xdd, 0xd1, 0xdc, 0x37},
		},
	}, "")
	if err != nil {
		t.Fatalf("selectAutoNetwork() error = %v", err)
	}
	if got.InterfaceName != "以太网" {
		t.Fatalf("selected = %q", got.InterfaceName)
	}
}

func TestSelectAutoNetworkHonorsAdapterHint(t *testing.T) {
	got, err := selectAutoNetwork([]networkCandidate{
		{
			Name:  "Ethernet",
			Flags: net.FlagUp,
			IP:    [4]byte{49, 140, 167, 234},
			MAC:   [6]byte{0x74, 0xd4, 0xdd, 0xd1, 0xdc, 0x37},
		},
		{
			Name:  "Wi-Fi",
			Flags: net.FlagUp,
			IP:    [4]byte{10, 1, 2, 3},
			MAC:   [6]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
		},
	}, "wi")
	if err != nil {
		t.Fatalf("selectAutoNetwork() error = %v", err)
	}
	if got.InterfaceName != "Wi-Fi" {
		t.Fatalf("selected = %q", got.InterfaceName)
	}
}

func TestSelectAutoNetworkRejectsMissingAdapterHint(t *testing.T) {
	_, err := selectAutoNetwork([]networkCandidate{
		{
			Name:  "Ethernet",
			Flags: net.FlagUp,
			IP:    [4]byte{49, 140, 167, 234},
			MAC:   [6]byte{0x74, 0xd4, 0xdd, 0xd1, 0xdc, 0x37},
		},
	}, "wi")
	if err == nil {
		t.Fatalf("selectAutoNetwork() expected error for missing adapter hint")
	}
}

func replaceLine(data, old, new string) string {
	return stringsReplace(data, old, new, 1)
}

func stringsReplace(s, old, new string, n int) string {
	if n == 0 || old == "" {
		return s
	}
	idx := -1
	for i := 0; i+len(old) <= len(s); i++ {
		if s[i:i+len(old)] == old {
			idx = i
			break
		}
	}
	if idx < 0 {
		return s
	}
	return s[:idx] + new + s[idx+len(old):]
}

func validConfig() string {
	return `
username = "student_id"
password = "password"
ip = "192.168.1.100"
mac = "00:11:22:33:44:55"
host_name = "windows-pc"
os_info = "Windows 11"

server_ip = "10.100.61.3"
server_port = 61440
bind_ip = "0.0.0.0"
bind_port = 61440

auth_version = "6800"
keepalive_version = "dc02"
first_heartbeat_version = "0f27"
extra_heartbeat_version = "db02"

primary_dns = "0.0.0.0"
dhcp_server = "0.0.0.0"

debug_hex_dump = false
receive_timeout_ms = 2000
retry_count = 3
heartbeat_interval_seconds = 20
`
}
