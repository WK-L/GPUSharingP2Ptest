package main

import (
	"time"

	protocol "github.com/libp2p/go-libp2p/core/protocol"
)

const (
	filesPushProtocol = protocol.ID("/files/push/1.0.0")
	peerInfoProtocol  = protocol.ID("/peer/info/1.0.0")
	outboxDir         = "./outbox"
	receivedDir       = "./received"
	defaultRendezvous = "/gpusharingp2ptest/files/receiver/v1"
	lanReceiverTTL    = 8 * time.Second
	dhtReceiverTTL    = 45 * time.Second
	broadcastInterval = 2 * time.Second
)
