package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	host "github.com/libp2p/go-libp2p/core/host"
)

func startWebServer(node host.Host, router *kaddht.IpfsDHT, webHost string, webPort string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/" {
			w.Header().Set("content-type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(appPage))
			return
		}

		if r.Method == http.MethodGet && r.URL.Path == "/api/state" {
			sendJSON(w, http.StatusOK, buildState(node, router, webPort))
			return
		}

		if r.Method == http.MethodPost && r.URL.Path == "/api/mode" {
			var body modeRequest
			if err := readJSON(r, &body); err != nil {
				sendError(w, err)
				return
			}
			if body.Mode != "sender" && body.Mode != "receiver" {
				sendError(w, errors.New("mode must be sender or receiver"))
				return
			}
			state.mu.Lock()
			state.mode = body.Mode
			if strings.TrimSpace(body.Name) != "" {
				state.name = strings.TrimSpace(body.Name)
			}
			state.mu.Unlock()
			sendJSON(w, http.StatusOK, buildState(node, router, webPort))
			return
		}

		if r.Method == http.MethodPost && r.URL.Path == "/api/files" {
			var body filesRequest
			if err := readJSON(r, &body); err != nil {
				sendError(w, err)
				return
			}
			if err := os.MkdirAll(outboxDir, 0755); err != nil {
				sendError(w, err)
				return
			}
			for _, file := range body.Files {
				data, err := base64.StdEncoding.DecodeString(file.Data)
				if err != nil {
					sendError(w, err)
					return
				}
				if err := os.WriteFile(filepath.Join(outboxDir, safeFileName(file.Name)), data, 0644); err != nil {
					sendError(w, err)
					return
				}
			}
			sendJSON(w, http.StatusOK, buildState(node, router, webPort))
			return
		}

		if r.Method == http.MethodDelete && r.URL.Path == "/api/files" {
			name := safeFileName(r.URL.Query().Get("name"))
			_ = os.Remove(filepath.Join(outboxDir, name))
			sendJSON(w, http.StatusOK, buildState(node, router, webPort))
			return
		}

		if r.Method == http.MethodPost && r.URL.Path == "/api/send" {
			var body sendRequest
			if err := readJSON(r, &body); err != nil {
				sendError(w, err)
				return
			}
			files, err := sendOutbox(node, body)
			if err != nil {
				sendError(w, err)
				return
			}
			sendJSON(w, http.StatusOK, map[string]any{"files": files})
			return
		}

		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/received/") {
			name := safeFileName(strings.TrimPrefix(r.URL.Path, "/received/"))
			http.ServeFile(w, r, filepath.Join(receivedDir, name))
			return
		}

		sendJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	})

	addr := net.JoinHostPort(webHost, webPort)
	fmt.Println("Web UI: http://127.0.0.1:" + webPort)
	for _, url := range webURLs(webPort) {
		fmt.Println("LAN Web UI:", url)
	}
	log.Fatal(http.ListenAndServe(addr, mux))
}
