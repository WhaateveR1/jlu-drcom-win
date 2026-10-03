//go:build windows

package transport

import (
	"encoding/binary"
	"fmt"
	"net"
	"syscall"
)

func pinInterface(conn *net.UDPConn, ip net.IP) error {
	if ip.IsUnspecified() || ip.IsLoopback() || ip.To4() == nil {
		return nil
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return err
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			network, ok := addr.(*net.IPNet)
			if !ok || !network.IP.Equal(ip) {
				continue
			}
			raw, err := conn.SyscallConn()
			if err != nil {
				return err
			}
			// IP_UNICAST_IF takes an interface index in network byte order.
			var index [4]byte
			binary.BigEndian.PutUint32(index[:], uint32(iface.Index))
			var socketErr error
			err = raw.Control(func(fd uintptr) {
				socketErr = syscall.SetsockoptInt(syscall.Handle(fd), syscall.IPPROTO_IP, 31, int(binary.NativeEndian.Uint32(index[:])))
			})
			if err != nil {
				return err
			}
			return socketErr
		}
	}
	return fmt.Errorf("no interface owns %s", ip)
}
