package transport

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestCancellationInterruptsReadAndDoesNotPoisonNextExchange(t *testing.T) {
	server, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	tr, err := NewTransport(&net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)}, server.LocalAddr().(*net.UDPAddr), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer tr.Close()
	for i := 0; i < 10; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		timer := time.AfterFunc(20*time.Millisecond, cancel)
		start := time.Now()
		_, err = tr.Exchange(ctx, []byte{1}, nil)
		timer.Stop()
		cancel()
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		if time.Since(start) > time.Second {
			t.Fatal("cancellation waited for socket timeout")
		}
	}
	// Replies to previous requests are irrelevant to this probe.
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 32)
		for {
			n, peer, e := server.ReadFromUDP(buf)
			if e != nil {
				return
			}
			if n == 1 && buf[0] == 2 {
				server.WriteToUDP([]byte{9}, peer)
				return
			}
		}
	}()
	if _, err = tr.Exchange(context.Background(), []byte{2}, func(p []byte) (bool, error) { return len(p) == 1 && p[0] == 9, nil }); err != nil {
		t.Fatal(err)
	}
	<-done
}

func TestForeignPeerAndUnmatchedPacketsDoNotEndExchange(t *testing.T) {
	server, _ := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	defer server.Close()
	foreign, _ := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	defer foreign.Close()
	tr, err := NewTransport(&net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)}, server.LocalAddr().(*net.UDPAddr), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer tr.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 10)
		_, peer, e := server.ReadFromUDP(buf)
		if e != nil {
			return
		}
		foreign.WriteToUDP([]byte{9}, peer)
		server.WriteToUDP([]byte{8}, peer)
		server.WriteToUDP([]byte{9}, peer)
	}()
	if _, err := tr.Exchange(context.Background(), []byte{1}, func(p []byte) (bool, error) { return len(p) == 1 && p[0] == 9, nil }); err != nil {
		t.Fatal(err)
	}
	<-done
}
