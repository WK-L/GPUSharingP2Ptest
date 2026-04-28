package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"time"

	host "github.com/libp2p/go-libp2p/core/host"
	network "github.com/libp2p/go-libp2p/core/network"
	peer "github.com/libp2p/go-libp2p/core/peer"
	peerstore "github.com/libp2p/go-libp2p/core/peerstore"
	protocol "github.com/libp2p/go-libp2p/core/protocol"
	ma "github.com/multiformats/go-multiaddr"
)

func newPeerInfoHandler(node host.Host) network.StreamHandler {
	return func(s network.Stream) {
		defer s.Close()

		state.mu.Lock()
		response := peerInfoResponse{
			Name:          state.name,
			PeerID:        node.ID().String(),
			PeerType:      peerType(),
			Addrs:         announceAddrs(node),
			DeployEnabled: dockerDeployEnabled(),
		}
		state.mu.Unlock()

		if err := writeStreamJSON(s, response); err != nil {
			log.Println("peer info write error:", err)
		}
	}
}

func handleDeployRequest(s network.Stream) {
	defer s.Close()

	bytes, err := io.ReadAll(s)
	if err != nil {
		_ = writeStreamJSON(s, deployResponse{Message: "could not read deploy request"})
		log.Println("deploy read error:", err)
		return
	}

	var payload deployPayload
	if err := json.Unmarshal(bytes, &payload); err != nil {
		_ = writeStreamJSON(s, deployResponse{Message: "deploy request json is invalid"})
		log.Println("deploy json error:", err)
		return
	}

	result := executeDeploy(payload)
	if err := writeStreamJSON(s, result.response); err != nil {
		log.Println("deploy response write error:", err)
		return
	}

	if cleanupOutput, cleanupErr := cleanupDockerDeployment(result.projectName, result.composeFiles, result.deployDir); cleanupErr != nil {
		log.Println("deploy cleanup error:", cleanupErr)
	} else if cleanupOutput != "" {
		log.Println("deploy cleanup:", cleanupOutput)
	}
}

func sendDeployBundle(node host.Host, req deployRequest) (deployResponse, error) {
	addr := req.Addr
	if addr == "" {
		state.mu.Lock()
		peerNode, ok := state.peers[req.PeerID]
		state.mu.Unlock()
		if !ok {
			return deployResponse{}, errors.New("peer not found or no longer visible")
		}
		addr = peerNode.Addr
	}

	archive, err := readBundleFile(req.ArchiveName)
	if err != nil {
		return deployResponse{}, err
	}

	state.mu.Lock()
	sourceNode := &peerBrief{PeerID: node.ID().String(), Name: state.name}
	state.mu.Unlock()

	payload := deployPayload{
		ProjectName:   req.ProjectName,
		ComposeFile:   req.ComposeFile,
		ArtifactPaths: splitCSV(req.ArtifactPaths),
		RequestedAt:   time.Now().Format(time.RFC3339),
		Source:        sourceNode,
		Archive:       archive,
	}

	stream, err := newStreamToAddr(node, addr, deployProtocol)
	if err != nil {
		return deployResponse{}, err
	}
	defer stream.Close()

	if err := writeStreamJSON(stream, payload); err != nil {
		return deployResponse{}, err
	}
	if err := stream.CloseWrite(); err != nil {
		return deployResponse{}, err
	}

	responseBytes, err := io.ReadAll(io.LimitReader(stream, 32*1024*1024))
	if err != nil {
		return deployResponse{}, err
	}

	var response deployResponse
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		return deployResponse{}, err
	}
	savedArtifacts, err := saveReturnedArtifacts(response.ProjectName, response.Artifacts)
	if err != nil {
		return deployResponse{}, err
	}
	recordDeployResult(response, payload, savedArtifacts)
	if !response.OK {
		if response.Message == "" {
			response.Message = "deployment failed"
		}
		return response, errors.New(response.Message)
	}
	return response, nil
}

func recordDeployResult(response deployResponse, payload deployPayload, savedArtifacts []string) {
	status := "success"
	if !response.OK {
		status = "failed"
	}

	event := deployEvent{
		At:          time.Now().Format(time.RFC3339),
		Source:      payload.Source,
		ProjectName: fallback(response.ProjectName, payload.ProjectName),
		ArchiveName: payload.Archive.Name,
		Status:      status,
		Command:     response.Command,
		Output:      response.Output,
		Logs:        response.Logs,
		Artifacts:   savedArtifacts,
	}

	state.mu.Lock()
	state.deploys = append([]deployEvent{event}, state.deploys...)
	state.deploys = firstDeploys(state.deploys, 20)
	state.mu.Unlock()
}

func newStreamToAddr(node host.Host, addr string, proto protocol.ID) (network.Stream, error) {
	maddr, err := ma.NewMultiaddr(addr)
	if err != nil {
		return nil, err
	}
	info, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		return nil, err
	}
	node.Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.PermanentAddrTTL)
	return node.NewStream(context.Background(), info.ID, proto)
}

func fetchPeerInfo(ctx context.Context, node host.Host, info peer.AddrInfo) (*peerInfoResponse, error) {
	if info.ID == node.ID() {
		return nil, errors.New("cannot fetch peer info from self")
	}
	if len(info.Addrs) > 0 {
		node.Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.TempAddrTTL)
	}
	if err := node.Connect(ctx, info); err != nil {
		return nil, err
	}

	stream, err := node.NewStream(ctx, info.ID, peerInfoProtocol)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	bytes, err := io.ReadAll(io.LimitReader(stream, 64*1024))
	if err != nil {
		return nil, err
	}

	var response peerInfoResponse
	if err := json.Unmarshal(bytes, &response); err != nil {
		return nil, err
	}
	if response.PeerID == "" {
		response.PeerID = info.ID.String()
	}
	return &response, nil
}

func upsertPeerNodeFromPeerInfo(info *peerInfoResponse, source string) {
	if info == nil || len(info.Addrs) == 0 {
		return
	}

	state.mu.Lock()
	state.peers[info.PeerID] = peerNode{
		PeerID:        info.PeerID,
		Name:          fallback(info.Name, info.PeerID),
		PeerType:      fallback(info.PeerType, "renter"),
		Addr:          info.Addrs[0],
		Addrs:         info.Addrs,
		Source:        source,
		DeployEnabled: info.DeployEnabled,
		SeenAt:        time.Now(),
	}
	state.mu.Unlock()
}

func removePeerNode(peerID peer.ID, source string) {
	state.mu.Lock()
	if peerNode, ok := state.peers[peerID.String()]; ok && peerNode.Source == source {
		delete(state.peers, peerID.String())
	}
	state.mu.Unlock()
}
