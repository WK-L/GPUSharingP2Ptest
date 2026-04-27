package app

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

func readBundleFile(name string) (bundleFile, error) {
	name = safeFileName(name)
	bytes, err := os.ReadFile(filepath.Join(bundleDir, name))
	if err != nil {
		return bundleFile{}, err
	}
	return bundleFile{
		Name: name,
		Data: base64.StdEncoding.EncodeToString(bytes),
	}, nil
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
