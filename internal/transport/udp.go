package transport

import (
	"errors"
	"fmt"
	"net"
	"time"
)

const defaultReceiveBufferSize = 512

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
	return &Transport{
		conn:    conn,
		server:  serverAddr,
		timeout: timeout,
	}, nil
}

func (t *Transport) Exchange(packet []byte) ([]byte, error) {
	if _, err := t.conn.WriteToUDP(packet, t.server); err != nil {
		return nil, err
	}

	if err := t.conn.SetReadDeadline(time.Now().Add(t.timeout)); err != nil {
		return nil, err
	}
	buf := make([]byte, defaultReceiveBufferSize)
	n, addr, err := t.conn.ReadFromUDP(buf)
	if err != nil {
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			return nil, ErrTimeout
		}
		return nil, err
	}
	if !sameUDPAddr(addr, t.server) {
		return nil, fmt.Errorf("unexpected udp peer: got %s want %s", addr, t.server)
	}
	return append([]byte(nil), buf[:n]...), nil
}

func (t *Transport) Send(packet []byte) error {
	_, err := t.conn.WriteToUDP(packet, t.server)
	return err
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
