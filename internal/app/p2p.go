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

func newDeployStatusHandler() network.StreamHandler {
	return func(s network.Stream) {
		defer s.Close()

		bytes, err := io.ReadAll(io.LimitReader(s, 64*1024))
		if err != nil {
			_ = writeStreamJSON(s, deployStatusResponse{})
			return
		}

		var req deployStatusRequest
		if err := json.Unmarshal(bytes, &req); err != nil {
			_ = writeStreamJSON(s, deployStatusResponse{})
			return
		}

		event, ok := getDeployEventByKey(req.Key)
		_ = writeStreamJSON(s, deployStatusResponse{
			Found: ok,
			Event: event,
		})
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

	projectName := safeProjectName(payload.ProjectName, payload.Archive.Name)
	sourceName := "unknown source"
	if payload.Source != nil {
		sourceName = fallback(payload.Source.Name, payload.Source.PeerID)
	}
	log.Printf("deploy request received from %s for archive %s as project %s\n", sourceName, payload.Archive.Name, projectName)
	upsertDeployEvent(deployEvent{
		Key:         deployEventKey(payload),
		At:          time.Now().Format(time.RFC3339),
		Source:      payload.Source,
		ProjectName: projectName,
		ArchiveName: payload.Archive.Name,
		Status:      "received",
		Output:      "Provider received deployment request and is preparing execution.",
	})

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
	eventKey := deployEventKey(payload)
	upsertDeployEvent(deployEvent{
		Key:         eventKey,
		At:          time.Now().Format(time.RFC3339),
		Source:      sourceNode,
		ProjectName: safeProjectName(req.ProjectName, req.ArchiveName),
		ArchiveName: req.ArchiveName,
		Status:      "sent",
		Output:      "Renter sent deployment request and is waiting for provider status.",
	})

	stream, err := newStreamToAddr(node, addr, deployProtocol)
	if err != nil {
		return deployResponse{}, err
	}
	defer stream.Close()

	stopPolling := make(chan struct{})
	pollDone := make(chan struct{})
	go pollRemoteDeployStatus(node, addr, eventKey, payload, stopPolling, pollDone)

	if err := writeStreamJSON(stream, payload); err != nil {
		close(stopPolling)
		<-pollDone
		return deployResponse{}, err
	}
	if err := stream.CloseWrite(); err != nil {
		close(stopPolling)
		<-pollDone
		return deployResponse{}, err
	}

	responseBytes, err := io.ReadAll(io.LimitReader(stream, 32*1024*1024))
	close(stopPolling)
	<-pollDone
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

	upsertDeployEvent(deployEvent{
		Key:         deployEventKey(payload),
		At:          time.Now().Format(time.RFC3339),
		Source:      payload.Source,
		ProjectName: fallback(response.ProjectName, payload.ProjectName),
		ArchiveName: payload.Archive.Name,
		Status:      status,
		Command:     response.Command,
		Output:      response.Output,
		Logs:        response.Logs,
		Artifacts:   savedArtifacts,
	})
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

func fetchRemoteDeployStatus(node host.Host, addr string, key string) (*deployStatusResponse, error) {
	stream, err := newStreamToAddr(node, addr, deployStatusProtocol)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	if err := writeStreamJSON(stream, deployStatusRequest{Key: key}); err != nil {
		return nil, err
	}
	if err := stream.CloseWrite(); err != nil {
		return nil, err
	}

	bytes, err := io.ReadAll(io.LimitReader(stream, 256*1024))
	if err != nil {
		return nil, err
	}

	var response deployStatusResponse
	if err := json.Unmarshal(bytes, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func pollRemoteDeployStatus(node host.Host, addr string, eventKey string, payload deployPayload, stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		default:
		}

		response, err := fetchRemoteDeployStatus(node, addr, eventKey)
		if err == nil && response != nil && response.Found {
			event := response.Event
			event.Key = eventKey
			if event.Source == nil {
				event.Source = payload.Source
			}
			if event.ArchiveName == "" {
				event.ArchiveName = payload.Archive.Name
			}
			upsertDeployEvent(event)
			if event.Status == "success" || event.Status == "failed" {
				return
			}
		}

		select {
		case <-stop:
			return
		case <-ticker.C:
		}
	}
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
