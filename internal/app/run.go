package app

import (
	"context"
	"fmt"

	libp2p "github.com/libp2p/go-libp2p"
	ma "github.com/multiformats/go-multiaddr"
)

func Run() error {
	if err := loadDotEnv(".env"); err != nil {
		return err
	}

	ctx := context.Background()
	priv, keyPath, created, err := loadOrCreatePrivateKey(defaultKeyPath())
	if err != nil {
		return err
	}

	webHost := getenv("APP_WEB_HOST", "0.0.0.0")
	webPort := getenv("APP_WEB_PORT", "3300")
	p2pHost := getenv("APP_P2P_HOST", "0.0.0.0")
	p2pPort := getenv("APP_P2P_PORT", "0")
	discoveryGroup := getenv("APP_DISCOVERY_GROUP", "239.255.77.77")
	discoveryPort := getenvInt("APP_DISCOVERY_PORT", 50197)

	listenAddr, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%s", p2pHost, p2pPort))
	if err != nil {
		return err
	}

	options, relayPeers, err := libp2pOptions(priv, listenAddr)
	if err != nil {
		return err
	}

	node, err := libp2p.New(options...)
	if err != nil {
		return err
	}

	node.SetStreamHandler(peerInfoProtocol, newPeerInfoHandler(node))
	node.SetStreamHandler(deployProtocol, handleDeployRequest)

	fmt.Println("Peer ID:", node.ID().String())
	if created {
		fmt.Println("Created key:", keyPath)
	} else {
		fmt.Println("Loaded key:", keyPath)
	}
	for _, addr := range announceAddrs(node) {
		fmt.Println("P2P address:", addr)
	}

	connectToStaticPeers(ctx, node, relayPeers)
	router, err := setupDHT(ctx, node, relayPeers)
	if err != nil {
		return err
	}

	go startDHTDiscovery(ctx, node, router)
	go startMDNSDiscovery(ctx, node)
	go startDiscovery(ctx, node, discoveryGroup, discoveryPort)
	startWebServer(node, router, webHost, webPort)
	return nil
}
