package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

const githubRepo = "swalha1999/lazycron"

type selfUpdateMsg struct {
	newVersion string
	err        error
}

func selfUpdate(currentVersion string) tea.Cmd {
	return func() tea.Msg {
		// Refuse to update dev builds (e.g. go run .)
		if currentVersion == "dev" || currentVersion == "" {
			return selfUpdateMsg{err: fmt.Errorf("cannot self-update a dev build — install from a release first")}
		}

		// Fetch latest release info
		url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", githubRepo)
		resp, err := http.Get(url)
		if err != nil {
			return selfUpdateMsg{err: fmt.Errorf("failed to check for updates: %w", err)}
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return selfUpdateMsg{err: fmt.Errorf("failed to check for updates: HTTP %d", resp.StatusCode)}
		}

		var release struct {
			TagName string `json:"tag_name"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
			return selfUpdateMsg{err: fmt.Errorf("failed to parse release info: %w", err)}
		}

		latestVersion := strings.TrimPrefix(release.TagName, "v")
		currentClean := strings.TrimPrefix(currentVersion, "v")

		if latestVersion == currentClean {
			return selfUpdateMsg{newVersion: ""}
		}

		// Determine platform
		goos := runtime.GOOS
		goarch := runtime.GOARCH

		filename := fmt.Sprintf("lazycron_%s_%s_%s.tar.gz", latestVersion, goos, goarch)
		downloadURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", githubRepo, release.TagName, filename)

		// Download to temp dir
		tmpDir, err := os.MkdirTemp("", "lazycron-update-*")
		if err != nil {
			return selfUpdateMsg{err: fmt.Errorf("failed to create temp dir: %w", err)}
		}
		defer os.RemoveAll(tmpDir)

		tarPath := tmpDir + "/" + filename
		dlResp, err := http.Get(downloadURL)
		if err != nil {
			return selfUpdateMsg{err: fmt.Errorf("failed to download update: %w", err)}
		}
		defer dlResp.Body.Close()

		if dlResp.StatusCode != 200 {
			return selfUpdateMsg{err: fmt.Errorf("failed to download update: HTTP %d", dlResp.StatusCode)}
		}

		out, err := os.Create(tarPath)
		if err != nil {
			return selfUpdateMsg{err: fmt.Errorf("failed to create temp file: %w", err)}
		}
		if _, err := io.Copy(out, dlResp.Body); err != nil {
			out.Close()
			return selfUpdateMsg{err: fmt.Errorf("failed to download update: %w", err)}
		}
		out.Close()

		// Extract tar.gz
		extractCmd := exec.Command("tar", "-xzf", tarPath, "-C", tmpDir)
		if err := extractCmd.Run(); err != nil {
			return selfUpdateMsg{err: fmt.Errorf("failed to extract update: %w", err)}
		}

		// Replace current binary (resolve symlinks to find the real path)
		currentExe, err := os.Executable()
		if err != nil {
			return selfUpdateMsg{err: fmt.Errorf("failed to find current executable: %w", err)}
		}
		currentExe, err = filepath.EvalSymlinks(currentExe)
		if err != nil {
			return selfUpdateMsg{err: fmt.Errorf("failed to resolve executable path: %w", err)}
		}

		newBinary := tmpDir + "/lazycron"

		if err := os.Chmod(newBinary, 0755); err != nil {
			return selfUpdateMsg{err: fmt.Errorf("failed to set permissions: %w", err)}
		}

		// Replace: rename old binary out of the way, copy new one in
		oldPath := currentExe + ".old"
		os.Remove(oldPath)

		if err := os.Rename(currentExe, oldPath); err != nil {
			return selfUpdateMsg{err: fmt.Errorf("failed to replace binary (try running with sudo): %w", err)}
		}

		if err := copyBinary(newBinary, currentExe); err != nil {
			// Try to restore on failure
			os.Rename(oldPath, currentExe)
			return selfUpdateMsg{err: fmt.Errorf("failed to install new binary: %w", err)}
		}

		os.Remove(oldPath)

		return selfUpdateMsg{newVersion: release.TagName}
	}
}

func copyBinary(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
