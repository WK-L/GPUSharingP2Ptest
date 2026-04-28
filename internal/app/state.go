package app

import (
	"sort"
	"strings"
	"time"

	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	host "github.com/libp2p/go-libp2p/core/host"
)

var state = &appState{
	name:  hostName(),
	peers: make(map[string]peerNode),
}

func buildState(node host.Host, router *kaddht.IpfsDHT, webPort string) stateResponse {
	bundles, _ := listFiles(bundleDir)
	artifacts, _ := listFilesRecursive(artifactsDir)

	state.mu.Lock()
	defer state.mu.Unlock()
	prunePeersLocked()

	peers := make([]peerNode, 0, len(state.peers))
	for _, peerNode := range state.peers {
		peers = append(peers, peerNode)
	}
	sort.Slice(peers, func(i, j int) bool {
		return peers[i].Name < peers[j].Name
	})

	return stateResponse{
		Name:        state.name,
		PeerID:      node.ID().String(),
		PeerType:    peerType(),
		Addrs:       announceAddrs(node),
		Network:     buildNetworkStatus(node, router),
		WebURLs:     webURLs(webPort),
		Bundles:     bundles,
		Artifacts:   artifacts,
		Peers:       peers,
		Deployments: append([]deployEvent{}, firstDeploys(state.deploys, 12)...),
	}
}

func prunePeersLocked() {
	now := time.Now()
	for peerID, peerNode := range state.peers {
		ttl := peerTTL(peerNode.Source)
		if now.Sub(peerNode.SeenAt) > ttl {
			delete(state.peers, peerID)
		}
	}
}

func peerTTL(source string) time.Duration {
	if source == "DHT" {
		return dhtPeerTTL
	}
	return lanPeerTTL
}

func firstDeploys(items []deployEvent, count int) []deployEvent {
	if len(items) <= count {
		return items
	}
	return items[:count]
}

func upsertDeployEvent(event deployEvent) {
	state.mu.Lock()
	defer state.mu.Unlock()

	for i, existing := range state.deploys {
		if event.Key != "" && existing.Key == event.Key {
			state.deploys[i] = event
			return
		}
	}

	state.deploys = append([]deployEvent{event}, state.deploys...)
	state.deploys = firstDeploys(state.deploys, 20)
}

func getDeployEventByKey(key string) (deployEvent, bool) {
	state.mu.Lock()
	defer state.mu.Unlock()

	for _, event := range state.deploys {
		if key != "" && event.Key == key {
			return event, true
		}
	}
	return deployEvent{}, false
}

func deployEventKey(payload deployPayload) string {
	sourcePeerID := ""
	if payload.Source != nil {
		sourcePeerID = payload.Source.PeerID
	}
	return strings.Join([]string{
		payload.RequestedAt,
		sourcePeerID,
		payload.Archive.Name,
		payload.ProjectName,
	}, "|")
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
		DockerDeploy:       getenvBool("APP_DOCKER_DEPLOY_ENABLED", false),
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
