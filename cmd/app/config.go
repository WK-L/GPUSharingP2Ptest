package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	host "github.com/libp2p/go-libp2p/core/host"
	ma "github.com/multiformats/go-multiaddr"
)

func announceAddrs(node host.Host) []string {
	peerID := node.ID().String()
	addrs := node.Addrs()
	p2pAddr := ma.StringCast("/p2p/" + peerID)
	out := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		out = append(out, addr.Encapsulate(p2pAddr).String())
	}
	return out
}

func localIPv4s() []string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return []string{"127.0.0.1"}
	}
	var ips []string
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch value := addr.(type) {
			case *net.IPNet:
				ip = value.IP
			case *net.IPAddr:
				ip = value.IP
			}
			if ip == nil || ip.To4() == nil {
				continue
			}
			ips = append(ips, ip.To4().String())
		}
	}
	if len(ips) == 0 {
		return []string{"127.0.0.1"}
	}
	return ips
}

func webURLs(webPort string) []string {
	ips := localIPv4s()
	urls := make([]string, 0, len(ips))
	for _, ip := range ips {
		urls = append(urls, "http://"+ip+":"+webPort)
	}
	return urls
}

func hostName() string {
	name, err := os.Hostname()
	if err != nil || name == "" {
		return "node"
	}
	return name
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("%s:%d: expected KEY=VALUE", path, lineNo)
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			return fmt.Errorf("%s:%d: empty key", path, lineNo)
		}

		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}

	return scanner.Err()
}

func getenv(key string, fallbackValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallbackValue
	}
	return value
}

func getenvInt(key string, fallbackValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallbackValue
	}
	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil {
		return fallbackValue
	}
	return parsed
}

func getenvBool(key string, fallbackValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallbackValue
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallbackValue
	}
	return parsed
}

func fallback(value string, fallbackValue string) string {
	if value == "" {
		return fallbackValue
	}
	return value
}
