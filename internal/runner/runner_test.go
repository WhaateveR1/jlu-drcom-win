package runner

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"jlu-drcom-win/internal/config"
	"jlu-drcom-win/internal/protocol"
	"jlu-drcom-win/internal/transport"
)

type fakeTransport struct {
	respond func([]byte) ([]byte, error)
	closed  bool
}

func (f *fakeTransport) Exchange(ctx context.Context, p []byte, _ func([]byte) (bool, error)) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return f.respond(p)
}
func (f *fakeTransport) Close() error { f.closed = true; return nil }

// Sanitized server shapes observed on JLU; all salts/tokens are synthetic.
func reply(p []byte) []byte {
	var out []byte
	switch p[0] {
	case 1:
		out = make([]byte, 76)
		copy(out, []byte{2, p[1], p[2], p[3], 1, 2, 3, 4})
	case 3:
		out = make([]byte, 45)
		out[0] = 4
		copy(out[23:], "synthetic-token!")
	case 6:
		out = make([]byte, 25)
		out[0] = 4
	case 0xff:
		out = make([]byte, 72)
		copy(out, []byte{7, 1, 16, 0, 6, 0})
	case 7:
		phase, size := byte(2), 40
		if p[5] == 3 {
			phase = 4
		} else if p[6] == 0x0f || p[6] == 0xdb {
			phase, size = 6, 272
		}
		out = make([]byte, size)
		copy(out, []byte{7, p[1], 0, 0, 0x0b, phase})
		binary.LittleEndian.PutUint16(out[2:4], uint16(size))
		copy(out[16:20], []byte{0xaa, 0xbb, 0xcc, 0xdd})
	}
	return out
}

func testConfig() config.Config {
	return config.Config{Username: "student", Password: "synthetic-password", IP: [4]byte{192, 0, 2, 10}, MAC: [6]byte{2, 1, 2, 3, 4, 5},
		AuthVersion: [2]byte{0x68, 0}, KeepAliveVersion: [2]byte{0xdc, 2}, FirstHeartbeatVersion: [2]byte{0x0f, 0x27}, ExtraHeartbeatVersion: [2]byte{0xdb, 2},
		ReceiveTimeout: time.Second, HeartbeatInterval: 20 * time.Second}
}

func TestSessionLifecycleAndSequenceWrap(t *testing.T) {
	f := &fakeTransport{respond: func(p []byte) ([]byte, error) { return reply(p), nil }}
	r := New(testConfig(), f, nil, nil)
	s, err := r.Login(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 140; i++ {
		if err := r.HeartbeatOnce(context.Background(), &s); err != nil {
			t.Fatal(i, err)
		}
	}
	if s.HeartbeatCount < 256 {
		t.Fatal("sequence wrap not exercised")
	}
	if s.HeartbeatToken != [4]byte{0xaa, 0xbb, 0xcc, 0xdd} {
		t.Fatal("token not updated")
	}
	if err := r.Logout(context.Background(), &s); err != nil {
		t.Fatal(err)
	}
	if r.State() != StateStopped {
		t.Fatal(r.State())
	}
}

func TestRejectedHeartbeatDoesNotAdvanceSession(t *testing.T) {
	f := &fakeTransport{respond: func([]byte) ([]byte, error) { return bytes.Repeat([]byte{5}, 40), nil }}
	r := New(testConfig(), f, nil, nil)
	s := protocol.Session{HeartbeatCount: 1, HeartbeatToken: [4]byte{1, 2, 3, 4}}
	before := s
	if err := r.HeartbeatOnce(context.Background(), &s); err == nil {
		t.Fatal("accepted rejection")
	}
	if s != before {
		t.Fatal("invalid reply changed session")
	}
}

func TestRunCancellationLogsOutAndCloses(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logout := false
	f := &fakeTransport{respond: func(p []byte) ([]byte, error) {
		if p[0] == 7 && p[5] == 3 {
			cancel()
		}
		if p[0] == 6 {
			logout = true
		}
		return reply(p), nil
	}}
	r := New(testConfig(), f, nil, nil)
	if err := r.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if !logout {
		t.Fatal("missing logout")
	}
	r.Close()
	if !f.closed {
		t.Fatal("not closed")
	}
}

func TestDebugLogsContainMetadataOnly(t *testing.T) {
	var log bytes.Buffer
	cfg := testConfig()
	cfg.DebugHexDump = true
	r := New(cfg, &fakeTransport{respond: func(p []byte) ([]byte, error) { return reply(p), nil }}, nil, slog.New(slog.NewTextHandler(&log, &slog.HandlerOptions{Level: slog.LevelDebug})))
	s, err := r.Login(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := r.HeartbeatOnce(context.Background(), &s); err != nil {
		t.Fatal(err)
	}
	if err := r.Logout(context.Background(), &s); err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{cfg.Username, cfg.Password, "synthetic-token!", "hex=", "aa bb cc dd"} {
		if strings.Contains(log.String(), secret) {
			t.Fatalf("sensitive log field: %s", secret)
		}
	}
	if !strings.Contains(log.String(), "response_bytes=") {
		t.Fatal("missing metadata")
	}
}

func TestLateDuplicateDatagramsDoNotBreakSession(t *testing.T) {
	server, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	done := make(chan error, 1)
	go func() {
		_ = server.SetDeadline(time.Now().Add(5 * time.Second))
		buf := make([]byte, 4096)
		var previous []byte
		for i := 0; i < 8; i++ {
			n, peer, err := server.ReadFromUDP(buf)
			if err != nil {
				done <- err
				return
			}
			if previous != nil {
				if _, err = server.WriteToUDP(previous, peer); err != nil {
					done <- err
					return
				}
			}
			previous = reply(buf[:n])
			if _, err = server.WriteToUDP(previous, peer); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	tr, err := transport.NewTransport(&net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)}, server.LocalAddr().(*net.UDPAddr), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer tr.Close()
	r := New(testConfig(), tr, nil, nil)
	s, err := r.Login(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err = r.HeartbeatOnce(context.Background(), &s); err != nil {
		t.Fatal(err)
	}
	if err = r.Logout(context.Background(), &s); err != nil {
		t.Fatal(err)
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
}

func TestSupervisorReloadsNetworkAndBoundsBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	loads, opens := 0, 0
	var delays []time.Duration
	s := Supervisor{
		Load: func() (config.Config, error) { loads++; c := testConfig(); c.IP[3] = byte(loads); return c, nil },
		Open: func(c config.Config) (Exchanger, error) {
			opens++
			if int(c.IP[3]) != opens {
				t.Fatal("stale IP")
			}
			return nil, io.ErrUnexpectedEOF
		},
		Wait: func(_ context.Context, d time.Duration) error {
			delays = append(delays, d)
			if len(delays) == 6 {
				cancel()
				return ctx.Err()
			}
			return nil
		},
	}
	if err := s.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if loads != 6 || opens != 6 {
		t.Fatal(loads, opens)
	}
	for i, want := range []time.Duration{5, 10, 20, 40, 60, 60} {
		if delays[i] != want*time.Second {
			t.Fatal(delays)
		}
	}
}

func TestSupervisorStopsOnInvalidConfigAndRejectedAccount(t *testing.T) {
	for _, invalid := range []bool{true, false} {
		opens := 0
		s := Supervisor{
			Load: func() (config.Config, error) {
				if invalid {
					return config.Config{}, errors.New("bad config")
				}
				return testConfig(), nil
			},
			Open: func(config.Config) (Exchanger, error) {
				opens++
				return &fakeTransport{respond: func(p []byte) ([]byte, error) {
					if p[0] == 3 {
						return []byte{5}, nil
					}
					return reply(p), nil
				}}, nil
			},
			Wait: func(context.Context, time.Duration) error { t.Fatal("permanent failure retried"); return nil },
		}
		if err := s.Run(context.Background()); err == nil {
			t.Fatal("expected failure")
		}
		if opens > 1 {
			t.Fatal(opens)
		}
	}
}

func TestHeartbeatFailureReloadsAndClosesOldTransport(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	loads := 0
	var opened []*fakeTransport
	s := Supervisor{
		Load: func() (config.Config, error) { loads++; return testConfig(), nil },
		Open: func(config.Config) (Exchanger, error) {
			attempt := loads
			f := &fakeTransport{respond: func(p []byte) ([]byte, error) {
				if attempt == 1 && p[0] == 0xff {
					return nil, transport.ErrTimeout
				}
				return reply(p), nil
			}}
			opened = append(opened, f)
			return f, nil
		},
		Wait: func(context.Context, time.Duration) error {
			if !opened[0].closed {
				t.Fatal("retry with old socket open")
			}
			return nil
		},
		OnState: func(state State) {
			if state == StateOnline {
				cancel()
			}
		},
	}
	if err := s.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if loads != 2 || len(opened) != 2 || !opened[1].closed {
		t.Fatal("reconnect lifecycle", loads)
	}
}
