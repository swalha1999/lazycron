package backend

import (
	"sort"
	"strings"

	sshclient "github.com/swalha1999/lazycron/ssh"
)

// RemoteDirLister lists directories on a remote server via SSH.
type RemoteDirLister struct {
	client  *sshclient.Client
	homeDir string // cached remote home directory
}

// NewRemoteDirLister creates a DirLister for a remote SSH server.
func NewRemoteDirLister(client *sshclient.Client) *RemoteDirLister {
	return &RemoteDirLister{client: client}
}

// ListDirs returns subdirectory names in the given path on the remote server.
func (r *RemoteDirLister) ListDirs(path string) ([]string, error) {
	// Use find with maxdepth 1 to list only direct subdirectories.
	// This is more reliable than ls for filtering directories only.
	escapedPath := strings.ReplaceAll(path, "'", "'\\''")
	cmd := "find '" + escapedPath + "' -maxdepth 1 -mindepth 1 -type d -o -type l 2>/dev/null | sort"
	output, err := r.client.Run(cmd)
	if err != nil {
		return nil, err
	}
	if output == "" {
		return nil, nil
	}

	var dirs []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Extract basename from full path
		parts := strings.Split(strings.TrimSuffix(line, "/"), "/")
		name := parts[len(parts)-1]
		if name != "" {
			dirs = append(dirs, name)
		}
	}
	sort.Strings(dirs)
	return dirs, nil
}

// HomeDir returns the remote user's home directory.
func (r *RemoteDirLister) HomeDir() (string, error) {
	if r.homeDir != "" {
		return r.homeDir, nil
	}
	home, err := r.client.Run("echo $HOME")
	if err != nil {
		return "", err
	}
	r.homeDir = strings.TrimSpace(home)
	return r.homeDir, nil
}
