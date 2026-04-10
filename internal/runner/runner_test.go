package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"jlu-drcom-win/internal/config"
	"jlu-drcom-win/internal/protocol"
	"jlu-drcom-win/internal/transport"
)

func TestRunnerLogin(t *testing.T) {
	cfg := testConfig()
	exchanger := &scriptedExchanger{t: t}
	r := New(cfg, exchanger, bytes.NewReader([]byte{0xaa, 0xbb, 0xcc, 0xdd}), slog.New(slog.NewTextHandler(io.Discard, nil)))

	session, err := r.Login(context.Background())
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if r.State() != StateOnline {
		t.Fatalf("state = %s", r.State())
	}
	if session.LoginSalt != [4]byte{0x01, 0x02, 0x03, 0x04} {
		t.Fatalf("login salt = % x", session.LoginSalt)
	}
	if !bytes.Equal(session.ServerDrcomIndicator[:], []byte("0123456789abcdef")) {
		t.Fatalf("server indicator = % x", session.ServerDrcomIndicator)
	}
	if exchanger.calls != 2 {
		t.Fatalf("exchange calls = %d", exchanger.calls)
	}
}

func TestRunnerLogout(t *testing.T) {
	cfg := testConfig()
	session := protocol.Session{
		ServerDrcomIndicator: [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
	}
	exchanger := &logoutExchanger{t: t}
	r := New(cfg, exchanger, bytes.NewReader([]byte{0xaa, 0xbb}), slog.New(slog.NewTextHandler(io.Discard, nil)))

	if err := r.Logout(context.Background(), &session); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if r.State() != StateStopped {
		t.Fatalf("state = %s", r.State())
	}
	if session.LogoutSalt != [4]byte{0x04, 0x03, 0x02, 0x01} {
		t.Fatalf("logout salt = % x", session.LogoutSalt)
	}
	if exchanger.calls != 2 {
		t.Fatalf("exchange calls = %d", exchanger.calls)
	}
}

func TestRunnerHeartbeatOnce(t *testing.T) {
	cfg := testConfig()
	session := protocol.Session{
		MD5Password:          [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
		ServerDrcomIndicator: [16]byte{15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0},
	}
	exchanger := &heartbeatExchanger{t: t}
	r := New(cfg, exchanger, bytes.NewReader([]byte{1, 2, 3, 4, 5, 6, 7, 8}), slog.New(slog.NewTextHandler(io.Discard, nil)))

	if err := r.HeartbeatOnce(context.Background(), &session); err != nil {
		t.Fatalf("HeartbeatOnce() error = %v", err)
	}
	if session.HeartbeatCount != 3 {
		t.Fatalf("heartbeat count = %d", session.HeartbeatCount)
	}
	if session.HeartbeatToken != [4]byte{0xaa, 0xbb, 0xcc, 0xdd} {
		t.Fatalf("heartbeat token = % x", session.HeartbeatToken)
	}
	if exchanger.calls != 4 {
		t.Fatalf("exchange calls = %d", exchanger.calls)
	}
}

func TestRunnerReconnectReopensTransportAndRelogins(t *testing.T) {
	cfg := testConfig()
	old := &closeTrackingExchanger{}
	factoryCalls := 0
	r := New(cfg, old, bytes.NewReader([]byte{0xaa, 0xbb, 0xcc, 0xdd}), slog.New(slog.NewTextHandler(io.Discard, nil)))
	r.factory = func() (Exchanger, error) {
		factoryCalls++
		return &scriptedExchanger{t: t}, nil
	}

	var session protocol.Session
	if err := r.Reconnect(context.Background(), &session); err != nil {
		t.Fatalf("Reconnect() error = %v", err)
	}
	if !old.closed {
		t.Fatalf("old transport was not closed")
	}
	if factoryCalls != 1 {
		t.Fatalf("factory calls = %d", factoryCalls)
	}
	if r.State() != StateOnline {
		t.Fatalf("state = %s", r.State())
	}
	if !bytes.Equal(session.ServerDrcomIndicator[:], []byte("0123456789abcdef")) {
		t.Fatalf("server indicator = % x", session.ServerDrcomIndicator)
	}
}

func TestRunnerRunLogsOutOnContextCancel(t *testing.T) {
	cfg := testConfig()
	cfg.HeartbeatInterval = time.Hour
	cfg.HeartbeatIntervalSecs = 3600
	ctx, cancel := context.WithCancel(context.Background())
	exchanger := &runCancelLogoutExchanger{t: t, cancel: cancel}
	rng := bytes.NewReader([]byte{
		0xaa, 0xbb,
		0xcc, 0xdd,
		1, 2, 3, 4,
		5, 6, 7, 8,
		0xee, 0xff,
	})
	r := New(cfg, exchanger, rng, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if err := r.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if r.State() != StateStopped {
		t.Fatalf("state = %s", r.State())
	}
	if exchanger.calls != 8 {
		t.Fatalf("exchange calls = %d", exchanger.calls)
	}
}

func TestRunnerHeartbeatOnceRetry(t *testing.T) {
	cfg := testConfig()
	cfg.RetryCount = 1
	session := protocol.Session{
		MD5Password:          [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
		ServerDrcomIndicator: [16]byte{15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0},
		HeartbeatCount:       1,
	}
	exchanger := &retryHeartbeatExchanger{t: t}
	r := New(cfg, exchanger, bytes.NewReader([]byte{5, 6, 7, 8}), slog.New(slog.NewTextHandler(io.Discard, nil)))

	if err := r.HeartbeatOnce(context.Background(), &session); err != nil {
		t.Fatalf("HeartbeatOnce() error = %v", err)
	}
	if exchanger.calls != 4 {
		t.Fatalf("exchange calls = %d", exchanger.calls)
	}
	if session.HeartbeatCount != 3 {
		t.Fatalf("heartbeat count = %d", session.HeartbeatCount)
	}
}

func TestRunnerLoginAndHeartbeatWithMockUDPServer(t *testing.T) {
	serverConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("ListenUDP() error = %v", err)
	}
	defer serverConn.Close()

	done := make(chan error, 1)
	go runMockAuthServer(serverConn, done)

	cfg := testConfig()
	cfg.ServerIP = [4]byte{127, 0, 0, 1}
	cfg.BindIP = [4]byte{127, 0, 0, 1}
	cfg.BindPort = 0
	tr, err := transport.NewTransport(&net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}, serverConn.LocalAddr().(*net.UDPAddr), time.Second)
	if err != nil {
		t.Fatalf("NewTransport() error = %v", err)
	}
	defer tr.Close()

	rng := bytes.NewReader([]byte{0xaa, 0xbb, 0xcc, 0xdd, 1, 2, 3, 4, 5, 6, 7, 8})
	r := New(cfg, tr, rng, slog.New(slog.NewTextHandler(io.Discard, nil)))

	session, err := r.Login(context.Background())
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if err := r.HeartbeatOnce(context.Background(), &session); err != nil {
		t.Fatalf("HeartbeatOnce() error = %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("mock server error = %v", err)
	}
	if session.HeartbeatCount != 3 {
		t.Fatalf("heartbeat count = %d", session.HeartbeatCount)
	}
}

type scriptedExchanger struct {
	t     *testing.T
	calls int
}

func (s *scriptedExchanger) Exchange(packet []byte) ([]byte, error) {
	s.calls++
	switch s.calls {
	case 1:
		if len(packet) != 20 {
			return nil, fmt.Errorf("challenge len = %d", len(packet))
		}
		if !bytes.Equal(packet[:6], []byte{0x01, 0x02, 0xaa, 0xbb, 0x68, 0x00}) {
			return nil, fmt.Errorf("challenge prefix = % x", packet[:6])
		}
		return []byte{0x02, 0x02, 0xaa, 0xbb, 0x01, 0x02, 0x03, 0x04}, nil
	case 2:
		if len(packet) < 334 {
			return nil, fmt.Errorf("login len = %d", len(packet))
		}
		if !bytes.Equal(packet[:2], []byte{0x03, 0x01}) {
			return nil, fmt.Errorf("login type = % x", packet[:2])
		}
		if !bytes.Equal(packet[81:85], []byte{192, 168, 1, 100}) {
			return nil, fmt.Errorf("login ip = % x", packet[81:85])
		}
		response := make([]byte, 39)
		copy(response[23:39], []byte("0123456789abcdef"))
		return response, nil
	default:
		s.t.Fatalf("unexpected exchange call %d", s.calls)
		return nil, nil
	}
}

type logoutExchanger struct {
	t     *testing.T
	calls int
}

func (l *logoutExchanger) Exchange(packet []byte) ([]byte, error) {
	l.calls++
	switch l.calls {
	case 1:
		if len(packet) != 20 {
			return nil, fmt.Errorf("logout challenge len = %d", len(packet))
		}
		if !bytes.Equal(packet[:6], []byte{0x01, 0x03, 0xaa, 0xbb, 0x68, 0x00}) {
			return nil, fmt.Errorf("logout challenge prefix = % x", packet[:6])
		}
		return []byte{0x02, 0x03, 0xaa, 0xbb, 0x04, 0x03, 0x02, 0x01}, nil
	case 2:
		if len(packet) != 80 {
			return nil, fmt.Errorf("logout len = %d", len(packet))
		}
		if !bytes.Equal(packet[:2], []byte{0x06, 0x01}) {
			return nil, fmt.Errorf("logout type = % x", packet[:2])
		}
		if !bytes.Equal(packet[64:80], []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}) {
			return nil, fmt.Errorf("logout indicator = % x", packet[64:80])
		}
		return []byte{0x06}, nil
	default:
		l.t.Fatalf("unexpected exchange call %d", l.calls)
		return nil, nil
	}
}

type closeTrackingExchanger struct {
	closed bool
}

func (c *closeTrackingExchanger) Exchange([]byte) ([]byte, error) {
	return nil, transport.ErrTimeout
}

func (c *closeTrackingExchanger) Close() error {
	c.closed = true
	return nil
}

type heartbeatExchanger struct {
	t     *testing.T
	calls int
}

func (h *heartbeatExchanger) Exchange(packet []byte) ([]byte, error) {
	h.calls++
	switch h.calls {
	case 1:
		if len(packet) != 38 || packet[0] != 0xff {
			return nil, fmt.Errorf("keepalive auth packet = % x", packet)
		}
		return []byte{0xff}, nil
	case 2:
		if !bytes.Equal(packet[:12], []byte{0x07, 0x00, 0x28, 0x00, 0x0b, 0x01, 0x0f, 0x27, 1, 2, 3, 4}) {
			return nil, fmt.Errorf("first heartbeat prefix = % x", packet[:12])
		}
		return []byte{0x07}, nil
	case 3:
		if !bytes.Equal(packet[:12], []byte{0x07, 0x01, 0x28, 0x00, 0x0b, 0x01, 0xdc, 0x02, 5, 6, 7, 8}) {
			return nil, fmt.Errorf("step1 prefix = % x", packet[:12])
		}
		response := make([]byte, 20)
		copy(response[16:20], []byte{0xaa, 0xbb, 0xcc, 0xdd})
		return response, nil
	case 4:
		if packet[1] != 0x02 || packet[5] != 0x03 {
			return nil, fmt.Errorf("step2 count/phase = % x", packet[:6])
		}
		if !bytes.Equal(packet[8:12], []byte{5, 6, 7, 8}) {
			return nil, fmt.Errorf("step2 random = % x", packet[8:12])
		}
		if !bytes.Equal(packet[16:20], []byte{0xaa, 0xbb, 0xcc, 0xdd}) {
			return nil, fmt.Errorf("step2 token = % x", packet[16:20])
		}
		return []byte{0x07}, nil
	default:
		h.t.Fatalf("unexpected exchange call %d", h.calls)
		return nil, nil
	}
}

type runCancelLogoutExchanger struct {
	t      *testing.T
	cancel context.CancelFunc
	calls  int
}

func (r *runCancelLogoutExchanger) Exchange(packet []byte) ([]byte, error) {
	r.calls++
	switch r.calls {
	case 1:
		if !bytes.Equal(packet[:6], []byte{0x01, 0x02, 0xaa, 0xbb, 0x68, 0x00}) {
			return nil, fmt.Errorf("login challenge prefix = % x", packet[:6])
		}
		return []byte{0x02, 0x02, 0xaa, 0xbb, 0x01, 0x02, 0x03, 0x04}, nil
	case 2:
		if len(packet) < 334 || !bytes.Equal(packet[:2], []byte{0x03, 0x01}) {
			return nil, fmt.Errorf("login packet = % x", packet[:min(len(packet), 8)])
		}
		response := make([]byte, 39)
		copy(response[23:39], []byte("0123456789abcdef"))
		return response, nil
	case 3:
		if len(packet) != 38 || packet[0] != 0xff {
			return nil, fmt.Errorf("keepalive packet = % x", packet[:min(len(packet), 8)])
		}
		return []byte{0xff}, nil
	case 4:
		if !bytes.Equal(packet[:8], []byte{0x07, 0x00, 0x28, 0x00, 0x0b, 0x01, 0x0f, 0x27}) {
			return nil, fmt.Errorf("first heartbeat = % x", packet[:min(len(packet), 12)])
		}
		return []byte{0x07}, nil
	case 5:
		if !bytes.Equal(packet[:8], []byte{0x07, 0x01, 0x28, 0x00, 0x0b, 0x01, 0xdc, 0x02}) {
			return nil, fmt.Errorf("step1 = % x", packet[:min(len(packet), 12)])
		}
		response := make([]byte, 20)
		copy(response[16:20], []byte{0xaa, 0xbb, 0xcc, 0xdd})
		return response, nil
	case 6:
		if packet[1] != 0x02 || packet[5] != 0x03 {
			return nil, fmt.Errorf("step2 = % x", packet[:min(len(packet), 12)])
		}
		r.cancel()
		return []byte{0x07}, nil
	case 7:
		if !bytes.Equal(packet[:6], []byte{0x01, 0x03, 0xee, 0xff, 0x68, 0x00}) {
			return nil, fmt.Errorf("logout challenge = % x", packet[:6])
		}
		return []byte{0x02, 0x03, 0xee, 0xff, 0x04, 0x03, 0x02, 0x01}, nil
	case 8:
		if len(packet) != 80 || !bytes.Equal(packet[:2], []byte{0x06, 0x01}) {
			return nil, fmt.Errorf("logout packet = % x", packet[:min(len(packet), 8)])
		}
		return []byte{0x06}, nil
	default:
		r.t.Fatalf("unexpected exchange call %d", r.calls)
		return nil, nil
	}
}

type retryHeartbeatExchanger struct {
	t     *testing.T
	calls int
}

func (r *retryHeartbeatExchanger) Exchange(packet []byte) ([]byte, error) {
	r.calls++
	switch r.calls {
	case 1:
		return nil, transport.ErrTimeout
	case 2:
		return []byte{0xff}, nil
	case 3:
		response := make([]byte, 20)
		copy(response[16:20], []byte{0x11, 0x22, 0x33, 0x44})
		return response, nil
	case 4:
		return []byte{0x07}, nil
	default:
		r.t.Fatalf("unexpected exchange call %d", r.calls)
		return nil, nil
	}
}

func runMockAuthServer(conn *net.UDPConn, done chan<- error) {
	buf := make([]byte, 512)
	for step := 1; step <= 6; step++ {
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			done <- err
			return
		}
		packet := append([]byte(nil), buf[:n]...)
		var response []byte
		switch step {
		case 1:
			if len(packet) != 20 || !bytes.Equal(packet[:2], []byte{0x01, 0x02}) {
				done <- fmt.Errorf("login challenge packet = % x", packet)
				return
			}
			response = []byte{0x02, 0x02, packet[2], packet[3], 0x01, 0x02, 0x03, 0x04}
		case 2:
			if len(packet) < 334 || !bytes.Equal(packet[:2], []byte{0x03, 0x01}) {
				done <- fmt.Errorf("login packet = % x", packet[:min(len(packet), 8)])
				return
			}
			response = make([]byte, 39)
			copy(response[23:39], []byte("0123456789abcdef"))
		case 3:
			if len(packet) != 38 || packet[0] != 0xff {
				done <- fmt.Errorf("keepalive auth packet = % x", packet[:min(len(packet), 8)])
				return
			}
			response = []byte{0xff}
		case 4:
			if len(packet) != 40 || !bytes.Equal(packet[:8], []byte{0x07, 0x00, 0x28, 0x00, 0x0b, 0x01, 0x0f, 0x27}) {
				done <- fmt.Errorf("first heartbeat packet = % x", packet[:min(len(packet), 12)])
				return
			}
			response = []byte{0x07}
		case 5:
			if len(packet) != 40 || !bytes.Equal(packet[:8], []byte{0x07, 0x01, 0x28, 0x00, 0x0b, 0x01, 0xdc, 0x02}) {
				done <- fmt.Errorf("heartbeat step1 packet = % x", packet[:min(len(packet), 12)])
				return
			}
			response = make([]byte, 20)
			copy(response[16:20], []byte{0xaa, 0xbb, 0xcc, 0xdd})
		case 6:
			if len(packet) != 40 || packet[1] != 0x02 || packet[5] != 0x03 {
				done <- fmt.Errorf("heartbeat step2 packet = % x", packet[:min(len(packet), 12)])
				return
			}
			if !bytes.Equal(packet[16:20], []byte{0xaa, 0xbb, 0xcc, 0xdd}) {
				done <- fmt.Errorf("heartbeat step2 token = % x", packet[16:20])
				return
			}
			response = []byte{0x07}
		}
		if _, err := conn.WriteToUDP(response, addr); err != nil {
			done <- err
			return
		}
	}
	done <- nil
}

func testConfig() config.Config {
	return config.Config{
		Username:              "u123",
		Password:              "passw0rd",
		IP:                    [4]byte{192, 168, 1, 100},
		MAC:                   [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
		HostName:              "win",
		OSInfo:                "Windows 11",
		ServerIP:              [4]byte{10, 100, 61, 3},
		ServerPort:            61440,
		BindIP:                [4]byte{0, 0, 0, 0},
		BindPort:              61440,
		AuthVersion:           [2]byte{0x68, 0x00},
		KeepAliveVersion:      [2]byte{0xdc, 0x02},
		FirstHeartbeatVersion: [2]byte{0x0f, 0x27},
		ExtraHeartbeatVersion: [2]byte{0xdb, 0x02},
		PrimaryDNS:            [4]byte{0, 0, 0, 0},
		DHCPServer:            [4]byte{0, 0, 0, 0},
		ReceiveTimeout:        time.Second,
		RetryCount:            0,
		HeartbeatInterval:     20 * time.Second,
		ReceiveTimeoutMillis:  1000,
		HeartbeatIntervalSecs: 20,
	}
}
