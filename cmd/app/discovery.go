package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"syscall"
	"time"

	host "github.com/libp2p/go-libp2p/core/host"
	"golang.org/x/net/ipv4"
)

func startDiscovery(ctx context.Context, node host.Host, group string, port int) {
	listenConfig := net.ListenConfig{Control: reuseUDPPort}
	packetConn, err := listenConfig.ListenPacket(ctx, "udp4", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Println("discovery listen error:", err)
		return
	}
	defer packetConn.Close()

	groupAddr := &net.UDPAddr{IP: net.ParseIP(group), Port: port}
	broadcastAddr := &net.UDPAddr{IP: net.IPv4bcast, Port: port}
	ipv4Conn := ipv4.NewPacketConn(packetConn)
	_ = ipv4Conn.SetMulticastLoopback(true)

	for _, iface := range multicastInterfaces() {
		if err := ipv4Conn.JoinGroup(&iface, groupAddr); err != nil {
			log.Println("could not join discovery group on", iface.Name+":", err)
		}
	}

	go func() {
		buf := make([]byte, 65536)
		for {
			n, _, err := packetConn.ReadFrom(buf)
			if err != nil {
				log.Println("discovery read error:", err)
				return
			}

			var announcement discoveryAnnouncement
			if err := json.Unmarshal(buf[:n], &announcement); err != nil {
				continue
			}
			if announcement.App != "p2ptest-lan" || announcement.Role != "receiver" {
				continue
			}
			if announcement.PeerID == node.ID().String() || len(announcement.Addrs) == 0 {
				continue
			}

			state.mu.Lock()
			state.receivers[announcement.PeerID] = receiverInfo{
				PeerID: announcement.PeerID,
				Name:   fallback(announcement.Name, announcement.PeerID),
				Addr:   announcement.Addrs[0],
				Addrs:  announcement.Addrs,
				Source: "LAN",
				SeenAt: time.Now(),
			}
			state.mu.Unlock()
		}
	}()

	ticker := time.NewTicker(broadcastInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			state.mu.Lock()
			mode := state.mode
			name := state.name
			state.mu.Unlock()
			if mode != "receiver" {
				continue
			}

			body, _ := json.Marshal(discoveryAnnouncement{
				App:     "p2ptest-lan",
				Role:    "receiver",
				PeerID:  node.ID().String(),
				Name:    name,
				Addrs:   announceAddrs(node),
				WebURLs: webURLs(getenv("APP_WEB_PORT", "3000")),
				At:      time.Now().Format(time.RFC3339),
			})
			_, _ = packetConn.WriteTo(body, groupAddr)
			_, _ = packetConn.WriteTo(body, broadcastAddr)
		}
	}
}
func multicastInterfaces() []net.Interface {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	out := make([]net.Interface, 0, len(interfaces))
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagMulticast == 0 {
			continue
		}
		if !interfaceHasIPv4(iface) {
			continue
		}
		out = append(out, iface)
	}
	return out
}

func interfaceHasIPv4(iface net.Interface) bool {
	addrs, err := iface.Addrs()
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		var ip net.IP
		switch value := addr.(type) {
		case *net.IPNet:
			ip = value.IP
		case *net.IPAddr:
			ip = value.IP
		}
		if ip != nil && ip.To4() != nil {
			return true
		}
	}
	return false
}

func reuseUDPPort(network string, address string, conn syscall.RawConn) error {
	var controlErr error
	if err := conn.Control(func(fd uintptr) {
		controlErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
		if controlErr != nil {
			return
		}
		controlErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEPORT, 1)
		if controlErr != nil {
			return
		}
		controlErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
	}); err != nil {
		return err
	}
	return controlErr
}
