package main

import (
	"time"

	protocol "github.com/libp2p/go-libp2p/core/protocol"
)

const (
	peerInfoProtocol  = protocol.ID("/peer/info/1.0.0")
	deployProtocol    = protocol.ID("/docker/deploy/1.0.0")
	bundleDir         = "./bundles"
	deploymentsDir    = "./deployments"
	defaultRendezvous = "/gpusharingp2ptest/docker/deploy/node/v1"
	lanPeerTTL        = 8 * time.Second
	dhtPeerTTL        = 45 * time.Second
	broadcastInterval = 2 * time.Second
)
