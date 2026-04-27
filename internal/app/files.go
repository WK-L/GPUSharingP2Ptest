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

func listFilesRecursive(dir string) ([]string, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	var names []string
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == dir || entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if strings.HasPrefix(entry.Name(), ".") {
			return nil
		}
		names = append(names, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(names)
	return names, nil
}

func saveReturnedArtifacts(projectName string, files []bundleFile) ([]string, error) {
	if len(files) == 0 {
		return nil, nil
	}

	projectDir := filepath.Join(artifactsDir, safeFileName(projectName))
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return nil, err
	}

	saved := make([]string, 0, len(files))
	for _, file := range files {
		relPath, err := safeRelativePath(fallback(file.Path, file.Name), file.Name)
		if err != nil {
			return nil, err
		}
		target := filepath.Join(projectDir, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return nil, err
		}
		data, err := base64.StdEncoding.DecodeString(file.Data)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(target, data, 0644); err != nil {
			return nil, err
		}
		saved = append(saved, filepath.ToSlash(filepath.Join(safeFileName(projectName), relPath)))
	}

	sort.Strings(saved)
	return saved, nil
}

func safeFileName(name string) string {
	clean := fileNameCleaner.ReplaceAllString(filepath.Base(name), "_")
	clean = strings.TrimSpace(clean)
	if clean == "" || clean == "." {
		return fmt.Sprintf("file-%d", time.Now().UnixMilli())
	}
	return clean
}
