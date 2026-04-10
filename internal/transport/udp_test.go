package transport

import (
	"bytes"
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestTransportExchangeWithMockUDPServer(t *testing.T) {
	serverConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("ListenUDP() error = %v", err)
	}
	defer serverConn.Close()

	done := make(chan error, 1)
	go func() {
		buf := make([]byte, 512)
		n, clientAddr, err := serverConn.ReadFromUDP(buf)
		if err != nil {
			done <- err
			return
		}
		if !bytes.Equal(buf[:n], []byte{0x01, 0x02}) {
			done <- errors.New("unexpected request payload")
			return
		}
		_, err = serverConn.WriteToUDP([]byte{0x02, 0x02, 0xaa, 0xbb}, clientAddr)
		done <- err
	}()

	tr, err := NewTransport(&net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}, serverConn.LocalAddr().(*net.UDPAddr), time.Second)
	if err != nil {
		t.Fatalf("NewTransport() error = %v", err)
	}
	defer tr.Close()

	resp, err := tr.Exchange([]byte{0x01, 0x02})
	if err != nil {
		t.Fatalf("Exchange() error = %v", err)
	}
	if !bytes.Equal(resp, []byte{0x02, 0x02, 0xaa, 0xbb}) {
		t.Fatalf("response = % x", resp)
	}
	if err := <-done; err != nil {
		t.Fatalf("mock server error = %v", err)
	}
}

func TestTransportExchangeTimeout(t *testing.T) {
	serverConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("ListenUDP() error = %v", err)
	}
	defer serverConn.Close()

	tr, err := NewTransport(&net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}, serverConn.LocalAddr().(*net.UDPAddr), 20*time.Millisecond)
	if err != nil {
		t.Fatalf("NewTransport() error = %v", err)
	}
	defer tr.Close()

	if _, err := tr.Exchange([]byte{0x01}); !IsTimeout(err) {
		t.Fatalf("Exchange() error = %v, want timeout", err)
	}
}

func TestRetryExchange(t *testing.T) {
	attempts := 0
	err := RetryExchange(context.Background(), 3, func() error {
		attempts++
		if attempts < 3 {
			return ErrTimeout
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RetryExchange() error = %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d", attempts)
	}
}
