package app

import (
	"sync"
	"time"
)

type appState struct {
	mu      sync.Mutex
	name    string
	peers   map[string]peerNode
	deploys []deployEvent
}

type peerNode struct {
	PeerID        string    `json:"peerId"`
	Name          string    `json:"name"`
	PeerType      string    `json:"peerType"`
	Addr          string    `json:"addr"`
	Addrs         []string  `json:"addrs"`
	Source        string    `json:"source"`
	DeployEnabled bool      `json:"deployEnabled"`
	SeenAt        time.Time `json:"-"`
}

type peerBrief struct {
	PeerID string `json:"peerId"`
	Name   string `json:"name"`
}

type bundleFile struct {
	Name string `json:"name"`
	Path string `json:"path,omitempty"`
	Data string `json:"data,omitempty"`
}

type deployEvent struct {
	Key         string     `json:"key,omitempty"`
	At          string     `json:"at"`
	Source      *peerBrief `json:"source,omitempty"`
	ProjectName string     `json:"projectName"`
	ArchiveName string     `json:"archiveName"`
	Status      string     `json:"status"`
	Output      string     `json:"output,omitempty"`
	Logs        string     `json:"logs,omitempty"`
	Artifacts   []string   `json:"artifacts,omitempty"`
}

type discoveryAnnouncement struct {
	App           string   `json:"app"`
	PeerID        string   `json:"peerId"`
	Name          string   `json:"name"`
	PeerType      string   `json:"peerType"`
	Addrs         []string `json:"addrs"`
	WebURLs       []string `json:"webUrls"`
	DeployEnabled bool     `json:"deployEnabled"`
	At            string   `json:"at"`
}

type peerInfoResponse struct {
	Name          string   `json:"name"`
	PeerID        string   `json:"peerId"`
	PeerType      string   `json:"peerType"`
	Addrs         []string `json:"addrs"`
	DeployEnabled bool     `json:"deployEnabled"`
}

type stateResponse struct {
	Name        string        `json:"name"`
	PeerID      string        `json:"peerId"`
	PeerType    string        `json:"peerType"`
	Addrs       []string      `json:"addrs"`
	Network     networkStatus `json:"network"`
	WebURLs     []string      `json:"webUrls"`
	Bundles     []string      `json:"bundles"`
	Artifacts   []string      `json:"artifacts"`
	Peers       []peerNode    `json:"peers"`
	Deployments []deployEvent `json:"deployments"`
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
	DockerDeploy       bool     `json:"dockerDeploy"`
}

type nodeRequest struct {
	Name string `json:"name"`
}

type bundlesRequest struct {
	Files []bundleFile `json:"files"`
}

type deployRequest struct {
	PeerID        string `json:"peerId"`
	Addr          string `json:"addr"`
	ArchiveName   string `json:"archiveName"`
	ProjectName   string `json:"projectName"`
	ComposeFile   string `json:"composeFile"`
	ArtifactPaths string `json:"artifactPaths"`
}

type deployPayload struct {
	ProjectName   string     `json:"projectName"`
	ComposeFile   string     `json:"composeFile"`
	ArtifactPaths []string   `json:"artifactPaths,omitempty"`
	RequestedAt   string     `json:"requestedAt"`
	Source        *peerBrief `json:"source,omitempty"`
	Archive       bundleFile `json:"archive"`
}

type deployResponse struct {
	OK          bool         `json:"ok"`
	Message     string       `json:"message"`
	Command     string       `json:"command,omitempty"`
	Output      string       `json:"output,omitempty"`
	Logs        string       `json:"logs,omitempty"`
	Artifacts   []bundleFile `json:"artifacts,omitempty"`
	ProjectName string       `json:"projectName,omitempty"`
	Directory   string       `json:"directory,omitempty"`
}

type deployStatusRequest struct {
	Key string `json:"key"`
}

type deployStatusResponse struct {
	Found bool        `json:"found"`
	Event deployEvent `json:"event"`
}
