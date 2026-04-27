//go:build windows

package app

import "net"

func discoveryListenConfig() net.ListenConfig {
	return net.ListenConfig{}
}
