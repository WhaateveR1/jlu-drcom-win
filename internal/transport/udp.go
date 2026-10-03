package transport

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"
)

const defaultReceiveBufferSize = 4096

var ErrTimeout = errors.New("udp receive timeout")

type Transport struct {
	conn    *net.UDPConn
	server  *net.UDPAddr
	timeout time.Duration
}

func NewTransport(bindAddr, serverAddr *net.UDPAddr, timeout time.Duration) (*Transport, error) {
	if bindAddr == nil {
		return nil, fmt.Errorf("bind address is nil")
	}
	if serverAddr == nil {
		return nil, fmt.Errorf("server address is nil")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("timeout must be positive")
	}

	conn, err := net.ListenUDP("udp", bindAddr)
	if err != nil {
		return nil, err
	}
	if err := pinInterface(conn, bindAddr.IP); err != nil {
		conn.Close()
		return nil, fmt.Errorf("select authentication interface: %w", err)
	}
	return &Transport{
		conn:    conn,
		server:  serverAddr,
		timeout: timeout,
	}, nil
}

// Exchange has a single owner. Unrelated/late datagrams never extend its deadline.
func (t *Transport) Exchange(ctx context.Context, packet []byte, match func([]byte) (bool, error)) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(t.timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if err := t.conn.SetDeadline(deadline); err != nil {
		return nil, err
	}
	finished := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		_ = t.conn.SetDeadline(time.Now())
		close(finished)
	})
	defer func() {
		if !stop() {
			<-finished
		}
		_ = t.conn.SetDeadline(time.Time{})
	}()
	if _, err := t.conn.WriteToUDP(packet, t.server); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}
	buf := make([]byte, defaultReceiveBufferSize)
	for {
		n, addr, err := t.conn.ReadFromUDP(buf)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if d, ok := ctx.Deadline(); ok && !time.Now().Before(d) {
				return nil, context.DeadlineExceeded
			}
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				return nil, ErrTimeout
			}
			return nil, err
		}
		if !sameUDPAddr(addr, t.server) {
			continue
		}
		if match != nil {
			ok, err := match(buf[:n])
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
		}
		return append([]byte(nil), buf[:n]...), nil
	}
}

func (t *Transport) Close() error {
	return t.conn.Close()
}

func sameUDPAddr(a, b *net.UDPAddr) bool {
	if a == nil || b == nil {
		return false
	}
	return a.Port == b.Port && a.IP.Equal(b.IP)
}
