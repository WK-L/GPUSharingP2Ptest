package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"time"

	host "github.com/libp2p/go-libp2p/core/host"
)

func startLocalDiscovery(ctx context.Context, node host.Host) {
	if !getenvBool("APP_LOCAL_DISCOVERY", true) {
		return
	}
	if err := os.MkdirAll(localDiscoveryDir(), 0700); err != nil {
		log.Println("local discovery setup error:", err)
		return
	}

	ticker := time.NewTicker(broadcastInterval)
	defer ticker.Stop()
	for {
		state.mu.Lock()
		mode := state.mode
		state.mu.Unlock()

		if mode == "receiver" {
			writeLocalAnnouncement(node)
		} else {
			removeLocalAnnouncement(node)
			readLocalAnnouncements(node)
		}

		select {
		case <-ctx.Done():
			removeLocalAnnouncement(node)
			return
		case <-ticker.C:
		}
	}
}

func writeLocalAnnouncement(node host.Host) {
	state.mu.Lock()
	name := state.name
	state.mu.Unlock()

	body, err := json.Marshal(discoveryAnnouncement{
		App:    "p2ptest-local",
		Role:   "receiver",
		PeerID: node.ID().String(),
		Name:   name,
		Addrs:  announceAddrs(node),
		At:     time.Now().Format(time.RFC3339),
	})
	if err != nil {
		log.Println("local discovery marshal error:", err)
		return
	}
	if err := os.WriteFile(localAnnouncementPath(node), body, 0600); err != nil {
		log.Println("local discovery write error:", err)
	}
}

func readLocalAnnouncements(node host.Host) {
	entries, err := os.ReadDir(localDiscoveryDir())
	if err != nil {
		return
	}
	now := time.Now()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		path := filepath.Join(localDiscoveryDir(), entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if now.Sub(info.ModTime()) > lanReceiverTTL {
			_ = os.Remove(path)
			continue
		}

		bytes, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var announcement discoveryAnnouncement
		if err := json.Unmarshal(bytes, &announcement); err != nil {
			continue
		}
		if announcement.App != "p2ptest-local" ||
			announcement.Role != "receiver" ||
			announcement.PeerID == node.ID().String() ||
			len(announcement.Addrs) == 0 {
			continue
		}

		state.mu.Lock()
		state.receivers[announcement.PeerID] = receiverInfo{
			PeerID: announcement.PeerID,
			Name:   fallback(announcement.Name, announcement.PeerID),
			Addr:   announcement.Addrs[0],
			Addrs:  announcement.Addrs,
			Source: "Local",
			SeenAt: now,
		}
		state.mu.Unlock()
	}
}

func removeLocalAnnouncement(node host.Host) {
	_ = os.Remove(localAnnouncementPath(node))
}

func localAnnouncementPath(node host.Host) string {
	return filepath.Join(localDiscoveryDir(), safeFileName(node.ID().String())+".json")
}

func localDiscoveryDir() string {
	return filepath.Join(os.TempDir(), "gpusharingp2ptest-discovery")
}
