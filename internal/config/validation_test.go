package config

import (
	"strings"
	"testing"
)

func TestStrictTOMLAndNumericBounds(t *testing.T) {
	for _, change := range [][2]string{
		{"heartbeat_interval_seconds = 20", "heartbeat_interval_seconds = 9223372037"},
		{"receive_timeout_ms = 2000", "receive_timeout_ms = 9223372036854775807"},
		{"retry_count = 3", "retry_count = 11"},
		{"debug_hex_dump = false", "debug_hex_dump = \"true\""},
		{"retry_count = 3", "retry_count = 3\nretry_count = 4"},
		{"retry_count = 3", "retry_cout = 3"},
	} {
		if _, err := Parse(strings.Replace(validConfig(), change[0], change[1], 1)); err == nil {
			t.Error("accepted", change[1])
		}
	}
}

func TestTOMLStringsAndErrorsDoNotLeakSecrets(t *testing.T) {
	data := strings.Replace(validConfig(), `password = "password"`, `password = 'literal#password'`, 1)
	c, err := Parse(data)
	if err != nil || c.Password != "literal#password" {
		t.Fatal("literal string parsing", err)
	}
	data = strings.Replace(validConfig(), `password = "password"`, `password = "private-secret`, 1)
	_, err = Parse(data)
	if err == nil || strings.Contains(err.Error(), "private-secret") {
		t.Fatal("parser leaked input", err)
	}
}

func TestMixedManualIPAutoMACCannotChooseDifferentAdapter(t *testing.T) {
	data := strings.Replace(validConfig(), `mac = "00:11:22:33:44:55"`, `mac = "auto"`, 1)
	_, err := ParseWithDetector(data, func(string) (NetworkInfo, error) {
		return NetworkInfo{IP: [4]byte{192, 0, 2, 20}, MAC: [6]byte{2, 1, 2, 3, 4, 5}}, nil
	})
	if err == nil {
		t.Fatal("mismatched adapter accepted")
	}
}
