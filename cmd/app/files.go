package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var fileNameCleaner = regexp.MustCompile(`[^a-zA-Z0-9._ -]`)

func readOutboxPayload() (filePayload, error) {
	names, err := listFiles(outboxDir)
	if err != nil {
		return filePayload{}, err
	}
	payload := filePayload{
		Files:     make([]fileItem, 0, len(names)),
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	for _, name := range names {
		bytes, err := os.ReadFile(filepath.Join(outboxDir, name))
		if err != nil {
			return filePayload{}, err
		}
		payload.Files = append(payload.Files, fileItem{
			Name: name,
			Data: base64.StdEncoding.EncodeToString(bytes),
		})
	}
	return payload, nil
}

func saveReceivedPayload(payload filePayload) ([]fileItem, error) {
	if err := os.MkdirAll(receivedDir, 0755); err != nil {
		return nil, err
	}

	saved := make([]fileItem, 0, len(payload.Files))
	for _, file := range payload.Files {
		name := safeFileName(file.Name)
		bytes, err := base64.StdEncoding.DecodeString(file.Data)
		if err != nil {
			return nil, err
		}
		path := filepath.Join(receivedDir, name)
		if err := os.WriteFile(path, bytes, 0644); err != nil {
			return nil, err
		}
		saved = append(saved, fileItem{Name: name, Path: path})
	}
	return saved, nil
}

func listFiles(dir string) ([]string, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names, nil
}

func safeFileName(name string) string {
	clean := fileNameCleaner.ReplaceAllString(filepath.Base(name), "_")
	clean = strings.TrimSpace(clean)
	if clean == "" || clean == "." {
		return fmt.Sprintf("file-%d", time.Now().UnixMilli())
	}
	return clean
}
