package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	libp2p "github.com/libp2p/go-libp2p"
	crypto "github.com/libp2p/go-libp2p/core/crypto"
	host "github.com/libp2p/go-libp2p/core/host"
	peer "github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

func libp2pOptions(privateKey crypto.PrivKey, listenAddr ma.Multiaddr) ([]libp2p.Option, []peer.AddrInfo, error) {
	announceAddrs, err := parseMultiaddrs(getenv("APP_ANNOUNCE_ADDRS", ""))
	if err != nil {
		return nil, nil, err
	}
	staticRelays, err := parsePeerAddrInfos(getenv("APP_STATIC_RELAYS", ""))
	if err != nil {
		return nil, nil, err
	}
	bootstrapPeers, err := parsePeerAddrInfos(getenv("APP_BOOTSTRAP_PEERS", ""))
	if err != nil {
		return nil, nil, err
	}

	options := []libp2p.Option{
		libp2p.Identity(privateKey),
		libp2p.ListenAddrs(listenAddr),
	}

	if len(announceAddrs) > 0 {
		options = append(options, libp2p.AddrsFactory(func(addrs []ma.Multiaddr) []ma.Multiaddr {
			return appendUniqueMultiaddrs(addrs, announceAddrs)
		}))
	}

	if getenvBool("APP_ENABLE_NAT_PORT_MAP", true) {
		options = append(options, libp2p.NATPortMap())
	}
	if getenvBool("APP_ENABLE_HOLE_PUNCHING", true) {
		options = append(options, libp2p.EnableHolePunching())
	}

	if len(staticRelays) > 0 {
		options = append(options, libp2p.EnableAutoRelayWithStaticRelays(staticRelays))
		if getenvBool("APP_FORCE_PRIVATE_REACHABILITY", true) {
			options = append(options, libp2p.ForceReachabilityPrivate())
		}
	}

	if getenvBool("APP_RELAY_SERVICE", false) {
		options = append(options,
			libp2p.EnableRelayService(),
			libp2p.EnableNATService(),
			libp2p.ForceReachabilityPublic(),
		)
	}

	peers := append([]peer.AddrInfo{}, staticRelays...)
	peers = append(peers, bootstrapPeers...)

	return options, peers, nil
}

func parseMultiaddrs(value string) ([]ma.Multiaddr, error) {
	items := splitCSV(value)
	addrs := make([]ma.Multiaddr, 0, len(items))
	for _, item := range items {
		addr, err := ma.NewMultiaddr(item)
		if err != nil {
			return nil, fmt.Errorf("invalid multiaddr %q: %w", item, err)
		}
		addrs = append(addrs, addr)
	}
	return addrs, nil
}

func parsePeerAddrInfos(value string) ([]peer.AddrInfo, error) {
	items := splitCSV(value)
	peers := make([]peer.AddrInfo, 0, len(items))
	for _, item := range items {
		addr, err := ma.NewMultiaddr(item)
		if err != nil {
			return nil, fmt.Errorf("invalid peer multiaddr %q: %w", item, err)
		}
		info, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil {
			return nil, fmt.Errorf("peer multiaddr must include /p2p/<peer-id> %q: %w", item, err)
		}
		peers = append(peers, *info)
	}
	return peers, nil
}

func appendUniqueMultiaddrs(base []ma.Multiaddr, extra []ma.Multiaddr) []ma.Multiaddr {
	seen := make(map[string]struct{}, len(base)+len(extra))
	out := make([]ma.Multiaddr, 0, len(base)+len(extra))
	for _, addr := range append(append([]ma.Multiaddr{}, base...), extra...) {
		key := addr.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, addr)
	}
	return out
}

func connectToStaticPeers(ctx context.Context, node host.Host, peers []peer.AddrInfo) {
	seen := make(map[peer.ID]struct{}, len(peers))
	for _, info := range peers {
		if _, ok := seen[info.ID]; ok {
			continue
		}
		seen[info.ID] = struct{}{}

		if err := node.Connect(ctx, info); err != nil {
			log.Printf("could not connect to static peer %s: %v\n", info.ID, err)
			continue
		}
		log.Printf("connected to static peer %s\n", info.ID)
	}
}

func splitCSV(value string) []string {
	raw := strings.Split(value, ",")
	items := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}
