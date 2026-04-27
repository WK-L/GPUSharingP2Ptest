package app

import (
	"encoding/base64"
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

		if r.Method == http.MethodPost && r.URL.Path == "/api/node" {
			var body nodeRequest
			if err := readJSON(r, &body); err != nil {
				sendError(w, err)
				return
			}
			state.mu.Lock()
			if strings.TrimSpace(body.Name) != "" {
				state.name = strings.TrimSpace(body.Name)
			}
			state.mu.Unlock()
			sendJSON(w, http.StatusOK, buildState(node, router, webPort))
			return
		}

		if r.Method == http.MethodPost && r.URL.Path == "/api/bundles" {
			var body bundlesRequest
			if err := readJSON(r, &body); err != nil {
				sendError(w, err)
				return
			}
			if err := os.MkdirAll(bundleDir, 0755); err != nil {
				sendError(w, err)
				return
			}
			for _, file := range body.Files {
				data, err := base64.StdEncoding.DecodeString(file.Data)
				if err != nil {
					sendError(w, err)
					return
				}
				if err := os.WriteFile(filepath.Join(bundleDir, safeFileName(file.Name)), data, 0644); err != nil {
					sendError(w, err)
					return
				}
			}
			sendJSON(w, http.StatusOK, buildState(node, router, webPort))
			return
		}

		if r.Method == http.MethodDelete && r.URL.Path == "/api/bundles" {
			name := safeFileName(r.URL.Query().Get("name"))
			_ = os.Remove(filepath.Join(bundleDir, name))
			return
		}

		if r.Method == http.MethodPost && r.URL.Path == "/api/deploy" {
			var body deployRequest
			if err := readJSON(r, &body); err != nil {
				sendError(w, err)
				return
			}
			response, err := sendDeployBundle(node, body)
			if err != nil {
				if response.Message != "" {
					sendJSON(w, http.StatusInternalServerError, response)
					return
				}
				sendError(w, err)
				return
			}
			sendJSON(w, http.StatusOK, response)
			return
		}

		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/artifacts/") {
			relPath, err := safeRelativePath(strings.TrimPrefix(r.URL.Path, "/artifacts/"), "")
			if err != nil {
				sendError(w, err)
				return
			}
			http.ServeFile(w, r, filepath.Join(artifactsDir, filepath.FromSlash(relPath)))
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
