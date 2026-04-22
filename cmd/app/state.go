package main

import (
	"sort"
	"strings"
	"time"

	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	host "github.com/libp2p/go-libp2p/core/host"
)

var state = &appState{
	mode:      "sender",
	name:      hostName(),
	receivers: make(map[string]receiverInfo),
}

func buildState(node host.Host, router *kaddht.IpfsDHT, webPort string) stateResponse {
	outbox, _ := listFiles(outboxDir)
	received, _ := listFiles(receivedDir)

	state.mu.Lock()
	defer state.mu.Unlock()
	pruneReceiversLocked()

	receivers := make([]receiverInfo, 0, len(state.receivers))
	for _, receiver := range state.receivers {
		receivers = append(receivers, receiver)
	}
	sort.Slice(receivers, func(i, j int) bool {
		return receivers[i].Name < receivers[j].Name
	})

	return stateResponse{
		Mode:      state.mode,
		Name:      state.name,
		PeerID:    node.ID().String(),
		Addrs:     announceAddrs(node),
		Network:   buildNetworkStatus(node, router),
		WebURLs:   webURLs(webPort),
		Outbox:    outbox,
		Received:  received,
		Receivers: receivers,
		Incoming:  append([]incomingEvent{}, firstIncoming(state.incoming, 12)...),
	}
}
func pruneReceiversLocked() {
	now := time.Now()
	for peerID, receiver := range state.receivers {
		ttl := receiverTTL(receiver.Source)
		if now.Sub(receiver.SeenAt) > ttl {
			delete(state.receivers, peerID)
		}
	}
}

func receiverTTL(source string) time.Duration {
	if source == "DHT" {
		return dhtReceiverTTL
	}
	return lanReceiverTTL
}

func firstIncoming(items []incomingEvent, count int) []incomingEvent {
	if len(items) <= count {
		return items
	}
	return items[:count]
}

func buildNetworkStatus(node host.Host, router *kaddht.IpfsDHT) networkStatus {
	circuitAddrs := circuitAddresses(announceAddrs(node))
	dhtPeers := 0
	if router != nil {
		dhtPeers = router.RoutingTable().Size()
	}
	staticRelays := splitCSV(getenv("APP_STATIC_RELAYS", ""))
	bootstrapPeers := splitCSV(getenv("APP_BOOTSTRAP_PEERS", ""))

	return networkStatus{
		RelayService:       getenvBool("APP_RELAY_SERVICE", false),
		StaticRelayCount:   len(staticRelays),
		BootstrapPeerCount: len(bootstrapPeers),
		DHTEnabled:         getenvBool("APP_DHT_ENABLED", true),
		DHTMode:            dhtModeName(),
		DHTPeers:           dhtPeers,
		ConnectedPeers:     len(node.Network().Peers()),
		Rendezvous:         rendezvousNamespace(),
		HasCircuitAddr:     len(circuitAddrs) > 0,
		CircuitAddrs:       circuitAddrs,
		RelayConfigured:    len(staticRelays) > 0 || getenvBool("APP_RELAY_SERVICE", false),
	}
}

func circuitAddresses(addrs []string) []string {
	out := make([]string, 0)
	for _, addr := range addrs {
		if strings.Contains(addr, "/p2p-circuit") {
			out = append(out, addr)
		}
	}
	return out
}
