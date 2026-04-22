package main

import (
	"sync"
	"time"
)

type appState struct {
	mu        sync.Mutex
	mode      string
	name      string
	receivers map[string]receiverInfo
	incoming  []incomingEvent
}

type receiverInfo struct {
	PeerID string    `json:"peerId"`
	Name   string    `json:"name"`
	Addr   string    `json:"addr"`
	Addrs  []string  `json:"addrs"`
	Source string    `json:"source"`
	SeenAt time.Time `json:"-"`
}

type peerBrief struct {
	PeerID string `json:"peerId"`
	Name   string `json:"name"`
}

type fileItem struct {
	Name string `json:"name"`
	Data string `json:"data,omitempty"`
	Path string `json:"path,omitempty"`
}

type filePayload struct {
	Files     []fileItem `json:"files"`
	CreatedAt string     `json:"createdAt"`
	Sender    *peerBrief `json:"sender,omitempty"`
}

type incomingEvent struct {
	At     string     `json:"at"`
	Sender *peerBrief `json:"sender,omitempty"`
	Files  []fileItem `json:"files,omitempty"`
}

type discoveryAnnouncement struct {
	App     string   `json:"app"`
	Role    string   `json:"role"`
	PeerID  string   `json:"peerId"`
	Name    string   `json:"name"`
	Addrs   []string `json:"addrs"`
	WebURLs []string `json:"webUrls"`
	At      string   `json:"at"`
}

type peerInfoResponse struct {
	Mode   string   `json:"mode"`
	Name   string   `json:"name"`
	PeerID string   `json:"peerId"`
	Addrs  []string `json:"addrs"`
}

type stateResponse struct {
	Mode      string          `json:"mode"`
	Name      string          `json:"name"`
	PeerID    string          `json:"peerId"`
	Addrs     []string        `json:"addrs"`
	Network   networkStatus   `json:"network"`
	WebURLs   []string        `json:"webUrls"`
	Outbox    []string        `json:"outbox"`
	Received  []string        `json:"received"`
	Receivers []receiverInfo  `json:"receivers"`
	Incoming  []incomingEvent `json:"incoming"`
}

type networkStatus struct {
	RelayService       bool     `json:"relayService"`
	StaticRelayCount   int      `json:"staticRelayCount"`
	BootstrapPeerCount int      `json:"bootstrapPeerCount"`
	DHTEnabled         bool     `json:"dhtEnabled"`
	DHTMode            string   `json:"dhtMode"`
	DHTPeers           int      `json:"dhtPeers"`
	ConnectedPeers     int      `json:"connectedPeers"`
	Rendezvous         string   `json:"rendezvous"`
	HasCircuitAddr     bool     `json:"hasCircuitAddr"`
	CircuitAddrs       []string `json:"circuitAddrs"`
	RelayConfigured    bool     `json:"relayConfigured"`
}

type modeRequest struct {
	Mode string `json:"mode"`
	Name string `json:"name"`
}

type filesRequest struct {
	Files []fileItem `json:"files"`
}

type sendRequest struct {
	PeerID string `json:"peerId"`
	Addr   string `json:"addr"`
}
