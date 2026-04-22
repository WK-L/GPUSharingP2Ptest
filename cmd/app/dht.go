package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/discovery"
	host "github.com/libp2p/go-libp2p/core/host"
	peer "github.com/libp2p/go-libp2p/core/peer"
	routingdisc "github.com/libp2p/go-libp2p/p2p/discovery/routing"
)

func setupDHT(ctx context.Context, node host.Host, bootstrapPeers []peer.AddrInfo) (*kaddht.IpfsDHT, error) {
	if !getenvBool("APP_DHT_ENABLED", true) {
		return nil, nil
	}

	mode, err := dhtMode()
	if err != nil {
		return nil, err
	}

	options := []kaddht.Option{kaddht.Mode(mode)}
	if len(bootstrapPeers) > 0 {
		options = append(options, kaddht.BootstrapPeers(bootstrapPeers...))
	}

	router, err := kaddht.New(ctx, node, options...)
	if err != nil {
		return nil, err
	}
	if err := router.Bootstrap(ctx); err != nil {
		return nil, err
	}

	log.Printf("DHT enabled in %s mode with rendezvous namespace %q\n", dhtModeName(), rendezvousNamespace())
	return router, nil
}

func startDHTDiscovery(ctx context.Context, node host.Host, router *kaddht.IpfsDHT) {
	if router == nil {
		return
	}

	discoveryClient := routingdisc.NewRoutingDiscovery(router)
	go advertiseReceivers(ctx, discoveryClient)
	go findReceivers(ctx, node, discoveryClient)
}

func advertiseReceivers(ctx context.Context, discoveryClient *routingdisc.RoutingDiscovery) {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for {
		state.mu.Lock()
		mode := state.mode
		state.mu.Unlock()
		if mode == "receiver" {
			advertiseOnce(ctx, discoveryClient)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func advertiseOnce(ctx context.Context, discoveryClient *routingdisc.RoutingDiscovery) {
	advertiseCtx, cancel := context.WithTimeout(ctx, 75*time.Second)
	defer cancel()

	ttl, err := discoveryClient.Advertise(advertiseCtx, rendezvousNamespace(), discovery.TTL(10*time.Minute))
	if err != nil {
		log.Println("DHT advertise error:", err)
		return
	}
	log.Printf("advertised receiver in DHT for %s\n", ttl)
}

func findReceivers(ctx context.Context, node host.Host, discoveryClient *routingdisc.RoutingDiscovery) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		state.mu.Lock()
		mode := state.mode
		state.mu.Unlock()
		if mode == "sender" {
			findReceiversOnce(ctx, node, discoveryClient)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func findReceiversOnce(ctx context.Context, node host.Host, discoveryClient *routingdisc.RoutingDiscovery) {
	findCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	peers, err := discoveryClient.FindPeers(findCtx, rendezvousNamespace(), discovery.Limit(30))
	if err != nil {
		log.Println("DHT find peers error:", err)
		return
	}

	for info := range peers {
		if info.ID == node.ID() || len(info.Addrs) == 0 {
			continue
		}

		connectCtx, connectCancel := context.WithTimeout(ctx, 5*time.Second)
		peerInfo, err := fetchPeerInfo(connectCtx, node, info)
		connectCancel()
		if err != nil {
			removeReceiver(info.ID, "DHT")
			continue
		}

		upsertReceiverFromPeerInfo(peerInfo, "DHT")
	}
}

func dhtMode() (kaddht.ModeOpt, error) {
	switch strings.ToLower(getenv("APP_DHT_MODE", defaultDHTModeName())) {
	case "auto":
		return kaddht.ModeAuto, nil
	case "auto-server", "autoserver":
		return kaddht.ModeAutoServer, nil
	case "server":
		return kaddht.ModeServer, nil
	case "client":
		return kaddht.ModeClient, nil
	default:
		return kaddht.ModeAutoServer, fmt.Errorf("APP_DHT_MODE must be auto, auto-server, server, or client")
	}
}

func dhtModeName() string {
	return strings.ToLower(getenv("APP_DHT_MODE", defaultDHTModeName()))
}

func defaultDHTModeName() string {
	if getenvBool("APP_RELAY_SERVICE", false) {
		return "server"
	}
	return "auto-server"
}

func rendezvousNamespace() string {
	return getenv("APP_RENDEZVOUS", defaultRendezvous)
}

func shortPeerName(id peer.ID) string {
	value := id.String()
	if len(value) <= 12 {
		return value
	}
	return "DHT " + value[:12]
}
