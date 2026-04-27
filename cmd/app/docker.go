package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const deployOutputLimit = 12000

func dockerDeployEnabled() bool {
	return getenvBool("APP_DOCKER_DEPLOY_ENABLED", false)
}

func dockerDeployAllowedPeers() []string {
	return splitCSV(getenv("APP_DOCKER_ALLOWED_PEERS", ""))
}

func dockerDeployRuntime() string {
	return strings.TrimSpace(getenv("APP_DOCKER_RUNTIME", ""))
}

func dockerWSLDistro() string {
	return strings.TrimSpace(getenv("APP_DOCKER_WSL_DISTRO", ""))
}

func executeDeploy(payload deployPayload, remotePeerID string) deployResponse {
	if !dockerDeployEnabled() {
		return deployResponse{Message: "docker deploy is disabled on this node"}
	}

	allowedPeers := dockerDeployAllowedPeers()
	if len(allowedPeers) > 0 && !containsString(allowedPeers, remotePeerID) {
		return deployResponse{Message: "source peer is not allowed to deploy to this node"}
	}

	expectedToken := strings.TrimSpace(getenv("APP_DOCKER_DEPLOY_TOKEN", ""))
	if expectedToken != "" && payload.Token != expectedToken {
		return deployResponse{Message: "invalid deploy token"}
	}

	if payload.Archive.Name == "" || payload.Archive.Data == "" {
		return deployResponse{Message: "deploy archive is required"}
	}

	projectName := safeProjectName(payload.ProjectName, payload.Archive.Name)
	composeFile, err := safeRelativePath(payload.ComposeFile, "docker-compose.yml")
	if err != nil {
		return deployResponse{Message: err.Error()}
	}

	archiveBytes, err := base64.StdEncoding.DecodeString(payload.Archive.Data)
	if err != nil {
		return deployResponse{Message: "could not decode deploy archive"}
	}

	if err := os.MkdirAll(deploymentsDir, 0755); err != nil {
		return deployResponse{Message: err.Error()}
	}

	deployDir, err := os.MkdirTemp(deploymentsDir, projectName+"-")
	if err != nil {
		return deployResponse{Message: err.Error()}
	}

	if err := extractDeployArchive(archiveBytes, payload.Archive.Name, deployDir); err != nil {
		return deployResponse{Message: err.Error(), Directory: deployDir}
	}

	composePath := filepath.Join(deployDir, filepath.FromSlash(composeFile))
	info, err := os.Stat(composePath)
	if err != nil {
		return deployResponse{Message: "compose file not found in bundle", Directory: deployDir}
	}
	if info.IsDir() {
		return deployResponse{Message: "compose file path points to a directory", Directory: deployDir}
	}

	command, err := newDockerDeployCommand(projectName, composePath, deployDir)
	if err != nil {
		return deployResponse{Message: err.Error(), Directory: deployDir}
	}
	output, err := command.CombinedOutput()
	message := "deployment completed"
	ok := true
	if err != nil {
		ok = false
		message = err.Error()
	}

	event := deployEvent{
		At:          time.Now().Format(time.RFC3339),
		Source:      payload.Source,
		ProjectName: projectName,
		ArchiveName: payload.Archive.Name,
		Status:      "success",
		Output:      trimOutput(string(output)),
	}
	if !ok {
		event.Status = "failed"
	}

	state.mu.Lock()
	state.deploys = append([]deployEvent{event}, state.deploys...)
	state.deploys = firstDeploys(state.deploys, 20)
	state.mu.Unlock()

	return deployResponse{
		OK:          ok,
		Message:     message,
		Output:      trimOutput(string(output)),
		ProjectName: projectName,
		Directory:   deployDir,
	}
}

func newDockerDeployCommand(projectName string, composePath string, deployDir string) (*exec.Cmd, error) {
	runtimeName := dockerDeployRuntime()
	args := []string{
		"compose",
		"-p",
		projectName,
		"-f",
		composePath,
		"up",
		"-d",
		"--build",
	}

	if runtime.GOOS == "windows" {
		wslArgs := make([]string, 0, len(args)+6)
		if distro := dockerWSLDistro(); distro != "" {
			wslArgs = append(wslArgs, "-d", distro)
		}
		wslArgs = append(wslArgs, "--cd", toWSLPath(deployDir))
		if runtimeName != "" {
			wslArgs = append(wslArgs, "env", "DOCKER_DEFAULT_RUNTIME="+runtimeName, "docker")
		} else {
			wslArgs = append(wslArgs, "docker")
		}
		wslArgs = append(wslArgs, args...)

		command := exec.Command("wsl", wslArgs...)
		command.Dir = deployDir
		return command, nil
	}

	command := exec.Command("docker", args...)
	command.Dir = deployDir
	if runtimeName != "" {
		command.Env = append(os.Environ(), "DOCKER_DEFAULT_RUNTIME="+runtimeName)
	}
	return command, nil
}

func toWSLPath(path string) string {
	slashed := filepath.ToSlash(path)
	if len(slashed) >= 2 && slashed[1] == ':' {
		drive := strings.ToLower(string(slashed[0]))
		rest := slashed[2:]
		return "/mnt/" + drive + rest
	}
	return slashed
}

func extractDeployArchive(archive []byte, archiveName string, destDir string) error {
	lower := strings.ToLower(archiveName)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return extractZipArchive(archive, destDir)
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return extractTarGzArchive(archive, destDir)
	case strings.HasSuffix(lower, ".tar"):
		return extractTarArchive(bytes.NewReader(archive), destDir)
	default:
		return errors.New("deploy bundle must be .zip, .tar.gz, .tgz, or .tar")
	}
}

func extractZipArchive(archive []byte, destDir string) error {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return err
	}

	for _, file := range reader.File {
		target, err := safeArchivePath(destDir, file.Name)
		if err != nil {
			return err
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		source, err := file.Open()
		if err != nil {
			return err
		}

		if err := writeFileFromReader(target, source, file.Mode()); err != nil {
			source.Close()
			return err
		}
		source.Close()
	}

	return nil
}

func extractTarGzArchive(archive []byte, destDir string) error {
	gzipReader, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	return extractTarArchive(gzipReader, destDir)
}

func extractTarArchive(reader io.Reader, destDir string) error {
	tarReader := tar.NewReader(reader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}

		target, err := safeArchivePath(destDir, header.Name)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			mode := os.FileMode(0644)
			if header.Mode != 0 {
				mode = os.FileMode(header.Mode)
			}
			if err := writeFileFromReader(target, tarReader, mode); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported archive entry type for %s", header.Name)
		}
	}
}

func writeFileFromReader(path string, reader io.Reader, mode os.FileMode) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, reader)
	return err
}

func safeArchivePath(destDir string, name string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." {
		return "", errors.New("archive entry path cannot be empty")
	}
	if filepath.IsAbs(clean) {
		return "", errors.New("archive entry cannot use absolute paths")
	}
	if strings.HasPrefix(clean, ".."+string(os.PathSeparator)) || clean == ".." {
		return "", errors.New("archive entry cannot escape deploy directory")
	}

	target := filepath.Join(destDir, clean)
	rel, err := filepath.Rel(destDir, target)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." {
		return "", errors.New("archive entry cannot escape deploy directory")
	}
	return target, nil
}

func safeRelativePath(path string, fallback string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		path = fallback
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	if filepath.IsAbs(clean) {
		return "", errors.New("compose file must be a relative path inside the bundle")
	}
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", errors.New("compose file must stay inside the bundle")
	}
	return filepath.ToSlash(clean), nil
}

func safeProjectName(value string, archiveName string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = strings.TrimSuffix(filepath.Base(archiveName), filepath.Ext(archiveName))
	}
	value = strings.ToLower(value)
	value = fileNameCleaner.ReplaceAllString(value, "-")
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.Trim(value, "-_.")
	if value == "" {
		return "deploy"
	}
	return value
}

func trimOutput(output string) string {
	output = strings.TrimSpace(output)
	if len(output) <= deployOutputLimit {
		return output
	}
	return output[:deployOutputLimit] + "\n...output truncated..."
}

func containsString(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}
