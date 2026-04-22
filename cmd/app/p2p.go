package main

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
			Mode:   state.mode,
			Name:   state.name,
			PeerID: node.ID().String(),
			Addrs:  announceAddrs(node),
		}
		state.mu.Unlock()

		if err := writeStreamJSON(s, response); err != nil {
			log.Println("peer info write error:", err)
		}
	}
}

func handleFilesPush(s network.Stream) {
	defer s.Close()

	bytes, err := io.ReadAll(s)
	if err != nil {
		log.Println("receiver push read error:", err)
		return
	}

	var payload filePayload
	if err := json.Unmarshal(bytes, &payload); err != nil {
		log.Println("receiver push json error:", err)
		return
	}

	saved, err := saveReceivedPayload(payload)
	if err != nil {
		log.Println("receiver save error:", err)
		return
	}

	event := incomingEvent{
		At:     time.Now().Format(time.RFC3339),
		Sender: payload.Sender,
		Files:  saved,
	}
	state.mu.Lock()
	state.incoming = append([]incomingEvent{event}, state.incoming...)
	state.incoming = firstIncoming(state.incoming, 20)
	state.mu.Unlock()
	log.Printf("received %d file(s)\n", len(saved))
}

func sendOutbox(node host.Host, req sendRequest) ([]fileItem, error) {
	addr := req.Addr
	if addr == "" {
		state.mu.Lock()
		receiver, ok := state.receivers[req.PeerID]
		state.mu.Unlock()
		if !ok {
			return nil, errors.New("receiver not found or no longer visible")
		}
		addr = receiver.Addr
	}

	payload, err := readOutboxPayload()
	if err != nil {
		return nil, err
	}
	state.mu.Lock()
	payload.Sender = &peerBrief{PeerID: node.ID().String(), Name: state.name}
	state.mu.Unlock()

	stream, err := newStreamToAddr(node, addr, filesPushProtocol)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	if err := writeStreamJSON(stream, payload); err != nil {
		return nil, err
	}

	return payload.Files, nil
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

func upsertReceiverFromPeerInfo(info *peerInfoResponse, source string) {
	if info == nil || info.Mode != "receiver" || len(info.Addrs) == 0 {
		return
	}

	state.mu.Lock()
	state.receivers[info.PeerID] = receiverInfo{
		PeerID: info.PeerID,
		Name:   fallback(info.Name, info.PeerID),
		Addr:   info.Addrs[0],
		Addrs:  info.Addrs,
		Source: source,
		SeenAt: time.Now(),
	}
	state.mu.Unlock()
}

func removeReceiver(peerID peer.ID, source string) {
	state.mu.Lock()
	if receiver, ok := state.receivers[peerID.String()]; ok && receiver.Source == source {
		delete(state.receivers, peerID.String())
	}
	state.mu.Unlock()
}
