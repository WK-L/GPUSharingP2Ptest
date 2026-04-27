package app

import (
	"context"
	"log"
	"time"

	host "github.com/libp2p/go-libp2p/core/host"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
)

type mdnsNotifee struct {
	ctx  context.Context
	node host.Host
}

func startMDNSDiscovery(ctx context.Context, node host.Host) {
	if !getenvBool("APP_MDNS_ENABLED", true) {
		return
	}

	service := mdns.NewMdnsService(node, "_gpusharingp2ptest._udp", &mdnsNotifee{
		ctx:  ctx,
		node: node,
	})
	if err := service.Start(); err != nil {
		log.Println("mDNS discovery error:", err)
		return
	}
	log.Println("mDNS discovery enabled")

	<-ctx.Done()
	_ = service.Close()
}

func (n *mdnsNotifee) HandlePeerFound(info peer.AddrInfo) {
	if info.ID == n.node.ID() {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(n.ctx, 5*time.Second)
		defer cancel()

		peerInfo, err := fetchPeerInfo(ctx, n.node, info)
		if err != nil {
			removePeerNode(info.ID, "mDNS")
			return
		}
		upsertPeerNodeFromPeerInfo(peerInfo, "mDNS")
	}()
}
