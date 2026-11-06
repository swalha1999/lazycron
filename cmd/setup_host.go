package cmd

import (
	"os"
	"path"
	"strings"

	"github.com/swalha1999/lazycron/cron"
	sshclient "github.com/swalha1999/lazycron/ssh"
)

// sshHost is a setup.Host on a server reached over SSH.
type sshHost struct {
	client *sshclient.Client
	osName string
	home   string
	user   string
}

func newSSHHost(c *sshclient.Client) (*sshHost, error) {
	h := &sshHost{client: c}
	for _, q := range []struct {
		cmd string
		dst *string
	}{{"uname -s", &h.osName}, {"echo $HOME", &h.home}, {"id -un", &h.user}} {
		out, err := c.Run(q.cmd)
		if err != nil {
			return nil, err
		}
		*q.dst = strings.TrimSpace(out)
	}
	h.osName = strings.ToLower(h.osName)
	return h, nil
}

func (h *sshHost) OS() string   { return h.osName }
func (h *sshHost) Home() string { return h.home }
func (h *sshHost) User() string { return h.user }

func (h *sshHost) Shell(command string) (string, error) { return h.client.Run(command) }

func (h *sshHost) ReadFile(p string) ([]byte, error) { return h.client.ReadFile(p) }

func (h *sshHost) WriteFile(p string, data []byte, mode os.FileMode) error {
	if _, err := h.client.Run("mkdir -p " + cron.ShellQuote(path.Dir(p))); err != nil {
		return err
	}
	return h.client.Upload(string(data), p, mode)
}
