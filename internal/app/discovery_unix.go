//go:build !windows

package app

import (
	"net"
	"syscall"

	"golang.org/x/sys/unix"
)

func discoveryListenConfig() net.ListenConfig {
	return net.ListenConfig{Control: reuseUDPPort}
}

func reuseUDPPort(network string, address string, conn syscall.RawConn) error {
	var controlErr error
	if err := conn.Control(func(fd uintptr) {
		controlErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
		if controlErr != nil {
			return
		}
		controlErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
		if controlErr != nil {
			return
		}
		controlErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_BROADCAST, 1)
	}); err != nil {
		return err
	}
	return controlErr
}
