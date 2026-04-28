package app

import (
	"archive/tar"
	"archive/zip"
	"bufio"
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
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const deployOutputLimit = 12000
const deployLogsLimit = 24000
const deployArtifactReturnLimit = 16 * 1024 * 1024

type dockerComposeFiles struct {
	basePath     string
	overridePath string
}

type deployExecutionResult struct {
	response     deployResponse
	composeFiles dockerComposeFiles
	deployDir    string
	projectName  string
}

func dockerDeployEnabled() bool {
	return getenvBool("APP_DOCKER_DEPLOY_ENABLED", false)
}

func dockerDeployRuntime() string {
	return strings.TrimSpace(getenv("APP_DOCKER_RUNTIME", ""))
}

func dockerWSLDistro() string {
	return strings.TrimSpace(getenv("APP_DOCKER_WSL_DISTRO", ""))
}

func executeDeploy(payload deployPayload) deployExecutionResult {
	eventKey := deployEventKey(payload)
	pushEvent := func(projectName string, archiveName string, status string, output string, logs string, artifacts []string) {
		upsertDeployEvent(deployEvent{
			Key:         eventKey,
			At:          time.Now().Format(time.RFC3339),
			Source:      payload.Source,
			ProjectName: projectName,
			ArchiveName: archiveName,
			Status:      status,
			Output:      output,
			Logs:        logs,
			Artifacts:   artifacts,
		})
	}
	fail := func(projectName string, archiveName string, deployDir string, message string) deployExecutionResult {
		pushEvent(projectName, archiveName, "failed", message, "", nil)
		return deployExecutionResult{response: deployResponse{Message: message, Directory: deployDir, ProjectName: projectName}}
	}

	if !dockerDeployEnabled() {
		return fail("", payload.Archive.Name, "", "docker deploy is disabled on this node")
	}

	if payload.Archive.Name == "" || payload.Archive.Data == "" {
		return fail("", payload.Archive.Name, "", "deploy archive is required")
	}

	projectName := safeProjectName(payload.ProjectName, payload.Archive.Name)
	defaultComposeFile := filepath.ToSlash(filepath.Join(defaultBundleRootName(payload.Archive.Name), "docker-compose.yml"))
	composeFile, err := safeRelativePath(payload.ComposeFile, defaultComposeFile)
	if err != nil {
		return fail(projectName, payload.Archive.Name, "", err.Error())
	}

	archiveBytes, err := base64.StdEncoding.DecodeString(payload.Archive.Data)
	if err != nil {
		return fail(projectName, payload.Archive.Name, "", "could not decode deploy archive")
	}

	if err := os.MkdirAll(deploymentsDir, 0755); err != nil {
		return fail(projectName, payload.Archive.Name, "", err.Error())
	}

	deployDir, err := os.MkdirTemp(deploymentsDir, projectName+"-")
	if err != nil {
		return fail(projectName, payload.Archive.Name, "", err.Error())
	}
	deployDir, err = filepath.Abs(deployDir)
	if err != nil {
		return fail(projectName, payload.Archive.Name, "", err.Error())
	}

	if err := extractDeployArchive(archiveBytes, payload.Archive.Name, deployDir); err != nil {
		return fail(projectName, payload.Archive.Name, deployDir, err.Error())
	}

	bundleRoot := filepath.Join(deployDir, filepath.FromSlash(defaultBundleRootName(payload.Archive.Name)))
	composePath := filepath.Join(deployDir, filepath.FromSlash(composeFile))
	info, err := os.Stat(composePath)
	if err != nil {
		return fail(projectName, payload.Archive.Name, deployDir, "compose file not found in bundle")
	}
	if info.IsDir() {
		return fail(projectName, payload.Archive.Name, deployDir, "compose file path points to a directory")
	}

	if err := checkDockerDeployPrerequisites(composePath, deployDir); err != nil {
		return fail(projectName, payload.Archive.Name, deployDir, err.Error())
	}

	composeFiles, err := prepareDockerComposeFiles(composePath, deployDir)
	if err != nil {
		return fail(projectName, payload.Archive.Name, deployDir, err.Error())
	}

	command, err := newDockerDeployCommand(projectName, composeFiles, deployDir)
	if err != nil {
		return fail(projectName, payload.Archive.Name, deployDir, err.Error())
	}
	commandLine := renderCommand(command)
	pushEvent(projectName, payload.Archive.Name, "running", "Provider started Docker deployment.", "Waiting for container output...", nil)
	output, err := runCommandStreaming(command, func(snapshot string) {
		pushEvent(projectName, payload.Archive.Name, "running", trimOutput(snapshot), trimLogs(snapshot), nil)
	})
	message := "deployment completed"
	ok := true
	if err != nil {
		ok = false
		message = err.Error()
	}
	logsOutput := collectComposeLogs(projectName, composeFiles, deployDir)
	artifacts, artifactErr := collectArtifacts(bundleRoot, payload.ArtifactPaths)
	if artifactErr != nil {
		ok = false
		message = artifactErr.Error()
	}

	status := "success"
	if !ok {
		status = "failed"
	}
	pushEvent(projectName, payload.Archive.Name, status, trimOutput(output), trimLogs(logsOutput), nil)

	return deployExecutionResult{
		response: deployResponse{
			OK:          ok,
			Message:     message,
			Command:     commandLine,
			Output:      trimOutput(output),
			Logs:        trimLogs(logsOutput),
			Artifacts:   artifacts,
			ProjectName: projectName,
			Directory:   deployDir,
		},
		composeFiles: composeFiles,
		deployDir:    deployDir,
		projectName:  projectName,
	}
}

func runCommandStreaming(command *exec.Cmd, onUpdate func(string)) (string, error) {
	if command == nil {
		return "", errors.New("command is required")
	}

	reader, writer := io.Pipe()
	command.Stdout = writer
	command.Stderr = writer

	var output string
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer reader.Close()

		var builder strings.Builder
		buffered := bufio.NewReader(reader)
		for {
			chunk, err := buffered.ReadString('\n')
			if chunk != "" {
				builder.WriteString(chunk)
				output = builder.String()
				if onUpdate != nil {
					onUpdate(output)
				}
			}
			if err != nil {
				rest, _ := io.ReadAll(buffered)
				if len(rest) > 0 {
					builder.Write(rest)
					output = builder.String()
					if onUpdate != nil {
						onUpdate(output)
					}
				}
				return
			}
		}
	}()

	if err := command.Start(); err != nil {
		_ = writer.Close()
		<-done
		return "", err
	}

	waitErr := command.Wait()
	_ = writer.Close()
	<-done
	if waitErr != nil {
		return output, formatCommandError(waitErr, []byte(output))
	}
	return output, nil
}

func checkDockerDeployPrerequisites(composePath string, deployDir string) error {
	runtimeName := dockerDeployRuntime()

	if runtime.GOOS == "windows" {
		if _, err := exec.LookPath("wsl"); err != nil {
			return errors.New("wsl is required on Windows Docker execution peers but was not found")
		}

		if _, err := runDockerPlatformCommand(deployDir, runtimeName, []string{"docker", "version"}, false); err != nil {
			return fmt.Errorf("could not run docker inside WSL: %w", err)
		}

		if runtimeName != "" {
			output, err := runDockerPlatformCommand(deployDir, runtimeName, []string{"docker", "info", "--format", "{{json .Runtimes}}"}, false)
			if err != nil {
				return fmt.Errorf("could not inspect docker runtimes inside WSL: %w", err)
			}
			if !dockerRuntimeExists(output, runtimeName) {
				return fmt.Errorf("docker runtime %q is not available inside WSL", runtimeName)
			}
		}

		wslComposePath := toWSLPath(composePath)
		if _, err := runDockerPlatformCommand(deployDir, runtimeName, []string{"test", "-f", wslComposePath}, true); err != nil {
			return fmt.Errorf("compose file %q is not visible inside WSL", wslComposePath)
		}
		return nil
	}

	if _, err := exec.LookPath("docker"); err != nil {
		return errors.New("docker was not found on this node")
	}
	if _, err := exec.Command("docker", "version").CombinedOutput(); err != nil {
		return fmt.Errorf("could not run docker: %w", err)
	}

	if runtimeName != "" {
		output, err := exec.Command("docker", "info", "--format", "{{json .Runtimes}}").CombinedOutput()
		if err != nil {
			return fmt.Errorf("could not inspect docker runtimes: %w", err)
		}
		if !dockerRuntimeExists(output, runtimeName) {
			return fmt.Errorf("docker runtime %q is not available on this node", runtimeName)
		}
	}

	return nil
}

func newDockerDeployCommand(projectName string, files dockerComposeFiles, deployDir string) (*exec.Cmd, error) {
	runtimeName := dockerDeployRuntime()
	args := dockerComposeCommandArgs(projectName, files, "up", "--build", "--abort-on-container-exit")

	if runtime.GOOS == "windows" {
		args = dockerComposeWSLArgs(projectName, files, "up", "--build", "--abort-on-container-exit")
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

func newDockerLogsCommand(projectName string, files dockerComposeFiles, deployDir string) *exec.Cmd {
	args := dockerComposeCommandArgs(projectName, files, "logs", "--no-color", "--tail", "200")

	if runtime.GOOS == "windows" {
		args = dockerComposeWSLArgs(projectName, files, "logs", "--no-color", "--tail", "200")
		wslArgs := make([]string, 0, len(args)+6)
		if distro := dockerWSLDistro(); distro != "" {
			wslArgs = append(wslArgs, "-d", distro)
		}
		wslArgs = append(wslArgs, "--cd", toWSLPath(deployDir), "docker")
		wslArgs = append(wslArgs, args...)
		command := exec.Command("wsl", wslArgs...)
		command.Dir = deployDir
		return command
	}

	command := exec.Command("docker", args...)
	command.Dir = deployDir
	return command
}

func newDockerCleanupCommand(projectName string, files dockerComposeFiles, deployDir string) *exec.Cmd {
	args := dockerComposeCommandArgs(projectName, files, "down", "--volumes", "--rmi", "all", "--remove-orphans")

	if runtime.GOOS == "windows" {
		wslArgs := make([]string, 0, len(args)+6)
		if distro := dockerWSLDistro(); distro != "" {
			wslArgs = append(wslArgs, "-d", distro)
		}
		wslArgs = append(wslArgs, "--cd", toWSLPath(deployDir), "docker")
		wslArgs = append(wslArgs, dockerComposeWSLArgs(projectName, files, "down", "--volumes", "--rmi", "all", "--remove-orphans")...)
		command := exec.Command("wsl", wslArgs...)
		command.Dir = deployDir
		return command
	}

	command := exec.Command("docker", args...)
	command.Dir = deployDir
	return command
}

func dockerComposeCommandArgs(projectName string, files dockerComposeFiles, action string, extraArgs ...string) []string {
	args := []string{"compose", "-p", projectName, "-f", files.basePath}
	if files.overridePath != "" {
		args = append(args, "-f", files.overridePath)
	}
	args = append(args, action)
	args = append(args, extraArgs...)
	return args
}

func dockerComposeWSLArgs(projectName string, files dockerComposeFiles, action string, extraArgs ...string) []string {
	wslFiles := dockerComposeFiles{
		basePath:     toWSLPath(files.basePath),
		overridePath: toWSLPath(files.overridePath),
	}
	return dockerComposeCommandArgs(projectName, wslFiles, action, extraArgs...)
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

func runDockerPlatformCommand(deployDir string, runtimeName string, args []string, shell bool) ([]byte, error) {
	if runtime.GOOS == "windows" {
		wslArgs := make([]string, 0, len(args)+6)
		if distro := dockerWSLDistro(); distro != "" {
			wslArgs = append(wslArgs, "-d", distro)
		}
		wslArgs = append(wslArgs, "--cd", toWSLPath(deployDir))
		if shell {
			commandLine := strings.Join(slices.Clone(args), " ")
			if runtimeName != "" {
				commandLine = "export DOCKER_DEFAULT_RUNTIME=" + shellEscape(runtimeName) + " && " + commandLine
			}
			wslArgs = append(wslArgs, "sh", "-lc", commandLine)
		} else {
			if runtimeName != "" {
				wslArgs = append(wslArgs, "env", "DOCKER_DEFAULT_RUNTIME="+runtimeName)
			}
			wslArgs = append(wslArgs, args...)
		}
		output, err := exec.Command("wsl", wslArgs...).CombinedOutput()
		if err != nil {
			return output, formatCommandError(err, output)
		}
		return output, nil
	}

	command := exec.Command(args[0], args[1:]...)
	command.Dir = deployDir
	if runtimeName != "" {
		command.Env = append(os.Environ(), "DOCKER_DEFAULT_RUNTIME="+runtimeName)
	}
	output, err := command.CombinedOutput()
	if err != nil {
		return output, formatCommandError(err, output)
	}
	return output, nil
}

func dockerRuntimeExists(output []byte, runtimeName string) bool {
	text := string(output)
	return strings.Contains(text, `"`+runtimeName+`"`) || strings.Contains(text, runtimeName+":")
}

func shellEscape(value string) string {
	value = strings.ReplaceAll(value, `'`, `'\''`)
	return "'" + value + "'"
}

func formatCommandError(err error, output []byte) error {
	text := strings.TrimSpace(string(output))
	if text == "" {
		return err
	}
	return fmt.Errorf("%w: %s", err, text)
}

func renderCommand(command *exec.Cmd) string {
	if command == nil {
		return ""
	}
	return strings.Join(append([]string{command.Path}, command.Args[1:]...), " ")
}

func collectComposeLogs(projectName string, files dockerComposeFiles, deployDir string) string {
	command := newDockerLogsCommand(projectName, files, deployDir)
	if command == nil {
		return ""
	}
	output, err := command.CombinedOutput()
	if err != nil && len(output) == 0 {
		return err.Error()
	}
	return string(output)
}

func trimLogs(output string) string {
	output = strings.TrimSpace(output)
	if len(output) <= deployLogsLimit {
		return output
	}
	return output[:deployLogsLimit] + "\n...logs truncated..."
}

func cleanupDockerDeployment(projectName string, files dockerComposeFiles, deployDir string) (string, error) {
	if strings.TrimSpace(projectName) == "" || strings.TrimSpace(deployDir) == "" || strings.TrimSpace(files.basePath) == "" {
		return "", nil
	}

	command := newDockerCleanupCommand(projectName, files, deployDir)
	output, commandErr := command.CombinedOutput()
	removeErr := os.RemoveAll(deployDir)

	var cleanupMessages []string
	text := strings.TrimSpace(string(output))
	if text != "" {
		cleanupMessages = append(cleanupMessages, "[cleanup]\n"+text)
	}

	if commandErr == nil && removeErr == nil {
		return strings.Join(cleanupMessages, "\n\n"), nil
	}

	var problems []string
	if commandErr != nil {
		problems = append(problems, formatCommandError(commandErr, output).Error())
	}
	if removeErr != nil {
		problems = append(problems, "could not remove deployment directory: "+removeErr.Error())
	}
	return strings.Join(cleanupMessages, "\n\n"), errors.New(strings.Join(problems, "; "))
}

func joinCommandOutputs(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return strings.Join(out, "\n\n")
}

func prepareDockerComposeFiles(composePath string, deployDir string) (dockerComposeFiles, error) {
	files := dockerComposeFiles{basePath: composePath}
	runtimeName := dockerDeployRuntime()
	if runtimeName == "" {
		return files, nil
	}

	overridePath, err := writeDockerRuntimeOverrideFile(composePath, deployDir, runtimeName)
	if err != nil {
		return dockerComposeFiles{}, err
	}
	files.overridePath = overridePath
	return files, nil
}

func writeDockerRuntimeOverrideFile(composePath string, deployDir string, runtimeName string) (string, error) {
	serviceNames, err := listComposeServiceNames(composePath)
	if err != nil {
		return "", err
	}
	if len(serviceNames) == 0 {
		return "", errors.New("compose file does not define any services")
	}

	type runtimeOverrideService struct {
		Runtime string `yaml:"runtime"`
	}
	type runtimeOverrideDoc struct {
		Services map[string]runtimeOverrideService `yaml:"services"`
	}

	override := runtimeOverrideDoc{
		Services: make(map[string]runtimeOverrideService, len(serviceNames)),
	}
	for _, serviceName := range serviceNames {
		override.Services[serviceName] = runtimeOverrideService{Runtime: runtimeName}
	}

	bytes, err := yaml.Marshal(override)
	if err != nil {
		return "", err
	}

	overridePath := filepath.Join(deployDir, ".runtime-override.compose.yaml")
	if err := os.WriteFile(overridePath, bytes, 0644); err != nil {
		return "", err
	}
	return overridePath, nil
}

func listComposeServiceNames(composePath string) ([]string, error) {
	type composeDocument struct {
		Services map[string]any `yaml:"services"`
	}

	bytes, err := os.ReadFile(composePath)
	if err != nil {
		return nil, err
	}

	var doc composeDocument
	if err := yaml.Unmarshal(bytes, &doc); err != nil {
		return nil, fmt.Errorf("could not parse compose file: %w", err)
	}

	names := make([]string, 0, len(doc.Services))
	for name := range doc.Services {
		names = append(names, name)
	}
	slices.Sort(names)
	return names, nil
}

func collectArtifacts(bundleRoot string, paths []string) ([]bundleFile, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	files := make([]bundleFile, 0)
	totalBytes := 0
	for _, rawPath := range paths {
		relPath, err := safeRelativePath(rawPath, "")
		if err != nil {
			return nil, err
		}
		absPath := filepath.Join(bundleRoot, filepath.FromSlash(relPath))
		info, err := os.Stat(absPath)
		if err != nil {
			return nil, fmt.Errorf("artifact path %q not found", relPath)
		}

		if info.IsDir() {
			err = filepath.WalkDir(absPath, func(path string, entry os.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if entry.IsDir() {
					return nil
				}
				return appendArtifactFile(bundleRoot, path, &files, &totalBytes)
			})
			if err != nil {
				return nil, err
			}
			continue
		}

		if err := appendArtifactFile(bundleRoot, absPath, &files, &totalBytes); err != nil {
			return nil, err
		}
	}

	return files, nil
}

func appendArtifactFile(bundleRoot string, absPath string, files *[]bundleFile, totalBytes *int) error {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return err
	}
	*totalBytes += len(data)
	if *totalBytes > deployArtifactReturnLimit {
		return fmt.Errorf("returned artifacts exceed %d bytes", deployArtifactReturnLimit)
	}
	relPath, err := filepath.Rel(bundleRoot, absPath)
	if err != nil {
		return err
	}
	*files = append(*files, bundleFile{
		Name: filepath.Base(absPath),
		Path: filepath.ToSlash(relPath),
		Data: base64.StdEncoding.EncodeToString(data),
	})
	return nil
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
		value = defaultBundleRootName(archiveName)
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

func defaultBundleRootName(archiveName string) string {
	name := filepath.Base(strings.TrimSpace(archiveName))
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".tar.gz"):
		return name[:len(name)-len(".tar.gz")]
	case strings.HasSuffix(lower, ".tgz"):
		return name[:len(name)-len(".tgz")]
	case strings.HasSuffix(lower, ".zip"):
		return name[:len(name)-len(".zip")]
	case strings.HasSuffix(lower, ".tar"):
		return name[:len(name)-len(".tar")]
	default:
		return strings.TrimSuffix(name, filepath.Ext(name))
	}
}

func trimOutput(output string) string {
	output = strings.TrimSpace(output)
	if len(output) <= deployOutputLimit {
		return output
	}
	return output[:deployOutputLimit] + "\n...output truncated..."
}
