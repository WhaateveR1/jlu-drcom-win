//go:build !windows

package transport

import "net"

func pinInterface(*net.UDPConn, net.IP) error { return nil }
