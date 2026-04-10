package protocol

import (
	"bytes"
	"crypto/md5"
	"testing"
	"time"
)

func TestLoginChallenge(t *testing.T) {
	packet := BuildLoginChallenge([2]byte{0x68, 0x00}, bytes.NewReader([]byte{0xaa, 0xbb}))
	if len(packet) != SizeChallenge {
		t.Fatalf("len = %d", len(packet))
	}
	wantPrefix := []byte{0x01, 0x02, 0xaa, 0xbb, 0x68, 0x00}
	if !bytes.Equal(packet[:6], wantPrefix) {
		t.Fatalf("packet prefix = % x", packet[:6])
	}
}

func TestLogoutChallenge(t *testing.T) {
	packet := BuildLogoutChallenge([2]byte{0x68, 0x00}, bytes.NewReader([]byte{0x11, 0x22}))
	wantPrefix := []byte{0x01, 0x03, 0x11, 0x22, 0x68, 0x00}
	if !bytes.Equal(packet[:6], wantPrefix) {
		t.Fatalf("packet prefix = % x", packet[:6])
	}
}

func TestChallengeResponseParsing(t *testing.T) {
	salt, err := ParseLoginChallengeResponse([]byte{0x02, 0x02, 0xaa, 0xbb, 0x01, 0x02, 0x03, 0x04})
	if err != nil {
		t.Fatalf("ParseLoginChallengeResponse() error = %v", err)
	}
	if salt != [4]byte{0x01, 0x02, 0x03, 0x04} {
		t.Fatalf("salt = % x", salt)
	}

	if _, err := ParseLogoutChallengeResponse([]byte{0x02, 0x02, 0xaa, 0xbb, 0x01, 0x02, 0x03, 0x04}); err == nil {
		t.Fatalf("ParseLogoutChallengeResponse() expected subtype error")
	}
}

func TestCryptoMatchesOriginalAlgorithms(t *testing.T) {
	if got := MD5([]byte("abc")); got != md5.Sum([]byte("abc")) {
		t.Fatalf("MD5 mismatch")
	}
	if got := XOR([]byte{0x0f, 0xf0}, []byte{0x33, 0x33}, 2); !bytes.Equal(got, []byte{0x3c, 0xc3}) {
		t.Fatalf("XOR = % x", got)
	}
	if got := ROR([]byte{0x12, 0x80}); !bytes.Equal(got, []byte{0x90, 0x04}) {
		t.Fatalf("ROR = % x", got)
	}
	if got := Checksum([]byte{1, 2, 3, 4, 5, 6, 7, 8}); got != [4]byte{0x20, 0x6d, 0xc6, 0x5e} {
		t.Fatalf("Checksum = % x", got)
	}
	if got := CRC([]byte{1, 2, 3, 4, 5, 6, 7, 8}); got != [4]byte{0x00, 0x38, 0x16, 0x00} {
		t.Fatalf("CRC = % x", got)
	}
}

func TestBuildLoginPacketFields(t *testing.T) {
	cfg := testProtocolConfig()
	session := &Session{LoginSalt: [4]byte{0x01, 0x02, 0x03, 0x04}}

	packet := BuildLoginPacket(cfg, session, bytes.NewReader([]byte{0xaa, 0xbb}))
	if len(packet) != 338 {
		t.Fatalf("login packet len = %d", len(packet))
	}
	if !bytes.Equal(packet[:4], []byte{0x03, 0x01, 0x00, byte(len(cfg.Username) + 20)}) {
		t.Fatalf("login header = % x", packet[:4])
	}

	md5aPlain := append([]byte{0x03, 0x01, 0x01, 0x02, 0x03, 0x04}, []byte(cfg.Password)...)
	md5a := md5.Sum(md5aPlain)
	if !bytes.Equal(packet[4:20], md5a[:]) {
		t.Fatalf("md5a = % x", packet[4:20])
	}
	if session.MD5Password != md5a {
		t.Fatalf("session md5 password = % x", session.MD5Password)
	}
	if !bytes.Equal(packet[20:20+len(cfg.Username)], []byte(cfg.Username)) {
		t.Fatalf("username field = %q", packet[20:56])
	}
	if packet[56] != 0x00 || packet[57] != 0x00 {
		t.Fatalf("control fields = % x", packet[56:58])
	}
	if !bytes.Equal(packet[58:64], XOR(cfg.MAC[:], md5a[:], 6)) {
		t.Fatalf("mac xor md5a = % x", packet[58:64])
	}
	if !bytes.Equal(packet[81:85], cfg.IP[:]) {
		t.Fatalf("ip field = % x", packet[81:85])
	}
	if !bytes.Equal(packet[190:192], []byte{0x68, 0x00}) || !bytes.Equal(packet[310:312], []byte{0x68, 0x00}) {
		t.Fatalf("auth version fields = % x / % x", packet[190:192], packet[310:312])
	}
	if packet[313] != byte(len(cfg.Password)) {
		t.Fatalf("password length = %d", packet[313])
	}

	rorLen := min(len(cfg.Password), 16)
	checksumStart := 316 + rorLen
	checksumPlain := make([]byte, 0, checksumStart+12)
	checksumPlain = append(checksumPlain, packet[:checksumStart]...)
	checksumPlain = append(checksumPlain, 0x01, 0x26, 0x07, 0x11, 0x00, 0x00)
	checksumPlain = append(checksumPlain, cfg.MAC[:]...)
	checksum := Checksum(checksumPlain)
	if !bytes.Equal(packet[checksumStart:checksumStart+4], checksum[:]) {
		t.Fatalf("checksum = % x want % x", packet[checksumStart:checksumStart+4], checksum)
	}
	if !bytes.Equal(packet[336:338], []byte{0xaa, 0xbb}) {
		t.Fatalf("login random trailer = % x", packet[336:338])
	}
}

func TestLoginResponseParsing(t *testing.T) {
	packet := make([]byte, 39)
	copy(packet[23:39], []byte("0123456789abcdef"))
	var session Session
	if err := ParseLoginResponse(packet, &session); err != nil {
		t.Fatalf("ParseLoginResponse() error = %v", err)
	}
	if !bytes.Equal(session.ServerDrcomIndicator[:], []byte("0123456789abcdef")) {
		t.Fatalf("server drcom indicator = % x", session.ServerDrcomIndicator)
	}
}

func TestKeepAliveAuth(t *testing.T) {
	session := Session{
		MD5Password:          [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
		ServerDrcomIndicator: [16]byte{15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0},
	}
	packet := BuildKeepAliveAuth(session, time.Unix(0x1234, 0))
	if len(packet) != SizeKeepAliveAuth {
		t.Fatalf("len = %d", len(packet))
	}
	if packet[0] != 0xff {
		t.Fatalf("packet[0] = %02x", packet[0])
	}
	if !bytes.Equal(packet[1:17], session.MD5Password[:]) {
		t.Fatalf("md5 password field = % x", packet[1:17])
	}
	if !bytes.Equal(packet[20:36], session.ServerDrcomIndicator[:]) {
		t.Fatalf("indicator field = % x", packet[20:36])
	}
	if !bytes.Equal(packet[36:38], []byte{0x34, 0x12}) {
		t.Fatalf("timestamp = % x", packet[36:38])
	}
}

func TestHeartbeatPackets(t *testing.T) {
	cfg := testProtocolConfig()
	session := Session{
		HeartbeatCount: 21,
		HeartbeatToken: [4]byte{0xde, 0xad, 0xbe, 0xef},
	}

	first := BuildFirstHeartbeat(cfg, Session{}, bytes.NewReader([]byte{1, 2, 3, 4}))
	if !bytes.Equal(first[:12], []byte{0x07, 0x00, 0x28, 0x00, 0x0b, 0x01, 0x0f, 0x27, 1, 2, 3, 4}) {
		t.Fatalf("first heartbeat prefix = % x", first[:12])
	}

	extra := BuildExtraHeartbeat(cfg, session, bytes.NewReader([]byte{5, 6, 7, 8}))
	if !bytes.Equal(extra[6:8], cfg.ExtraHeartbeatVersion[:]) {
		t.Fatalf("extra version = % x", extra[6:8])
	}
	if !bytes.Equal(extra[16:20], session.HeartbeatToken[:]) {
		t.Fatalf("extra token = % x", extra[16:20])
	}

	step1 := BuildHeartbeatStep1(cfg, session, bytes.NewReader([]byte{9, 10, 11, 12}))
	if !bytes.Equal(step1[8:12], []byte{9, 10, 11, 12}) {
		t.Fatalf("step1 random token = % x", step1[8:12])
	}

	response := make([]byte, 20)
	copy(response[16:20], []byte{0xaa, 0xbb, 0xcc, 0xdd})
	if err := ParseHeartbeatStep1Response(response, &session); err != nil {
		t.Fatalf("ParseHeartbeatStep1Response() error = %v", err)
	}
	if session.HeartbeatToken != [4]byte{0xaa, 0xbb, 0xcc, 0xdd} {
		t.Fatalf("heartbeat token = % x", session.HeartbeatToken)
	}

	randomToken := [4]byte{9, 10, 11, 12}
	step2 := BuildHeartbeatStep2(cfg, session, randomToken)
	if step2[5] != 0x03 {
		t.Fatalf("step2 phase = %02x", step2[5])
	}
	if !bytes.Equal(step2[8:12], randomToken[:]) {
		t.Fatalf("step2 random token = % x", step2[8:12])
	}
	if !bytes.Equal(step2[28:32], cfg.IP[:]) {
		t.Fatalf("step2 ip = % x", step2[28:32])
	}
	crcPlain := append([]byte{}, step2[:24]...)
	crcPlain = append(crcPlain, cfg.IP[:]...)
	crc := CRC(crcPlain)
	if !bytes.Equal(step2[24:28], crc[:]) {
		t.Fatalf("step2 crc = % x want % x", step2[24:28], crc)
	}
}

func TestBuildLogoutPacketFields(t *testing.T) {
	cfg := testProtocolConfig()
	session := Session{
		LogoutSalt:           [4]byte{0x04, 0x03, 0x02, 0x01},
		ServerDrcomIndicator: [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
	}
	packet := BuildLogoutPacket(cfg, session)
	if len(packet) != SizeLogout {
		t.Fatalf("logout len = %d", len(packet))
	}
	if !bytes.Equal(packet[:4], []byte{0x06, 0x01, 0x00, byte(len(cfg.Username) + 20)}) {
		t.Fatalf("logout header = % x", packet[:4])
	}

	md5aPlain := append([]byte{0x06, 0x01, 0x04, 0x03, 0x02, 0x01}, []byte(cfg.Password)...)
	md5a := md5.Sum(md5aPlain)
	if !bytes.Equal(packet[4:20], md5a[:]) {
		t.Fatalf("logout md5a = % x", packet[4:20])
	}
	if !bytes.Equal(packet[58:64], XOR(cfg.MAC[:], md5a[:], 6)) {
		t.Fatalf("logout mac xor md5a = % x", packet[58:64])
	}
	if !bytes.Equal(packet[64:80], session.ServerDrcomIndicator[:]) {
		t.Fatalf("logout indicator = % x", packet[64:80])
	}
}

func testProtocolConfig() Config {
	return Config{
		Username:              "u123",
		Password:              "passw0rd",
		IP:                    [4]byte{192, 168, 1, 100},
		MAC:                   [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
		HostName:              "win",
		OSInfo:                "Windows 11",
		PrimaryDNS:            [4]byte{0, 0, 0, 0},
		DHCPServer:            [4]byte{0, 0, 0, 0},
		AuthVersion:           [2]byte{0x68, 0x00},
		KeepAliveVersion:      [2]byte{0xdc, 0x02},
		FirstHeartbeatVersion: [2]byte{0x0f, 0x27},
		ExtraHeartbeatVersion: [2]byte{0xdb, 0x02},
	}
}
