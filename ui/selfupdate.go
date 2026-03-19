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
	// When sudo is needed, these are set instead of replacing directly
	needsSudo  bool
	tmpBinary  string
	targetPath string
}

type selfUpdateSudoMsg struct {
	newVersion string
	tmpDir     string
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

		// Download to temp dir (not cleaned up here if sudo is needed)
		tmpDir, err := os.MkdirTemp("", "lazycron-update-*")
		if err != nil {
			return selfUpdateMsg{err: fmt.Errorf("failed to create temp dir: %w", err)}
		}

		tarPath := tmpDir + "/" + filename
		dlResp, err := http.Get(downloadURL)
		if err != nil {
			os.RemoveAll(tmpDir)
			return selfUpdateMsg{err: fmt.Errorf("failed to download update: %w", err)}
		}
		defer dlResp.Body.Close()

		if dlResp.StatusCode != 200 {
			os.RemoveAll(tmpDir)
			return selfUpdateMsg{err: fmt.Errorf("failed to download update: HTTP %d", dlResp.StatusCode)}
		}

		out, err := os.Create(tarPath)
		if err != nil {
			os.RemoveAll(tmpDir)
			return selfUpdateMsg{err: fmt.Errorf("failed to create temp file: %w", err)}
		}
		if _, err := io.Copy(out, dlResp.Body); err != nil {
			out.Close()
			os.RemoveAll(tmpDir)
			return selfUpdateMsg{err: fmt.Errorf("failed to download update: %w", err)}
		}
		out.Close()

		// Extract tar.gz
		extractCmd := exec.Command("tar", "-xzf", tarPath, "-C", tmpDir)
		if err := extractCmd.Run(); err != nil {
			os.RemoveAll(tmpDir)
			return selfUpdateMsg{err: fmt.Errorf("failed to extract update: %w", err)}
		}

		// Resolve current binary path
		currentExe, err := os.Executable()
		if err != nil {
			os.RemoveAll(tmpDir)
			return selfUpdateMsg{err: fmt.Errorf("failed to find current executable: %w", err)}
		}
		currentExe, err = filepath.EvalSymlinks(currentExe)
		if err != nil {
			os.RemoveAll(tmpDir)
			return selfUpdateMsg{err: fmt.Errorf("failed to resolve executable path: %w", err)}
		}

		newBinary := tmpDir + "/lazycron"

		if err := os.Chmod(newBinary, 0755); err != nil {
			os.RemoveAll(tmpDir)
			return selfUpdateMsg{err: fmt.Errorf("failed to set permissions: %w", err)}
		}

		// Try direct copy first
		if err := directCopy(newBinary, currentExe); err == nil {
			os.RemoveAll(tmpDir)
			return selfUpdateMsg{newVersion: release.TagName}
		}

		// Need sudo — keep tmpDir alive for the next phase
		return selfUpdateMsg{
			newVersion: release.TagName,
			needsSudo:  true,
			tmpBinary:  newBinary,
			targetPath: currentExe,
		}
	}
}

// sudoInstall returns a tea.Cmd that pauses the TUI, runs sudo cp, and resumes.
func sudoInstall(newVersion, tmpBinary, targetPath string) tea.Cmd {
	tmpDir := filepath.Dir(tmpBinary)
	c := exec.Command("sudo", "cp", tmpBinary, targetPath)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		defer os.RemoveAll(tmpDir)
		if err != nil {
			return selfUpdateSudoMsg{err: fmt.Errorf("sudo install failed: %w", err)}
		}
		return selfUpdateSudoMsg{newVersion: newVersion}
	})
}

func directCopy(src, dst string) error {
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
