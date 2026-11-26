package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// execRecord is what the test server saw for one exec request.
type execRecord struct {
	cmd   string
	stdin string
}

// startExecServer starts an SSH server on localhost that accepts any public
// key, records each exec command with its stdin, and exits 0. It returns a
// client trusted by a known_hosts file under a temporary $HOME.
func startExecServer(t *testing.T) (*Client, <-chan execRecord) {
	t.Helper()
	hostSigner := newSigner(t)
	cfg := &ssh.ServerConfig{
		PublicKeyCallback: func(ssh.ConnMetadata, ssh.PublicKey) (*ssh.Permissions, error) { return nil, nil },
	}
	cfg.AddHostKey(hostSigner)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	records := make(chan execRecord, 4)
	go acceptLoop(ln, cfg, records)

	addr := ln.Addr().(*net.TCPAddr)
	keyPath := writeClientKey(t)
	writeKnownHosts(t, addr.String(), hostSigner.PublicKey())
	t.Setenv("SSH_AUTH_SOCK", "")
	c := NewClient("127.0.0.1", addr.Port, "tester", "", keyPath, false)
	t.Cleanup(func() { c.Close() })
	return c, records
}

func acceptLoop(ln net.Listener, cfg *ssh.ServerConfig, records chan<- execRecord) {
	for {
		nc, err := ln.Accept()
		if err != nil {
			return
		}
		go serveConn(nc, cfg, records)
	}
}

func serveConn(nc net.Conn, cfg *ssh.ServerConfig, records chan<- execRecord) {
	_, chans, reqs, err := ssh.NewServerConn(nc, cfg)
	if err != nil {
		return
	}
	go ssh.DiscardRequests(reqs)
	for nch := range chans {
		ch, chReqs, err := nch.Accept()
		if err != nil {
			continue
		}
		go serveExec(ch, chReqs, records)
	}
}

func serveExec(ch ssh.Channel, reqs <-chan *ssh.Request, records chan<- execRecord) {
	defer ch.Close()
	for req := range reqs {
		if req.Type != "exec" {
			req.Reply(false, nil)
			continue
		}
		var payload struct{ Command string }
		ssh.Unmarshal(req.Payload, &payload)
		req.Reply(true, nil)
		stdin, _ := io.ReadAll(ch)
		records <- execRecord{cmd: payload.Command, stdin: string(stdin)}
		ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
		return
	}
}

func newSigner(t *testing.T) ssh.Signer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	s, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func writeClientKey(t *testing.T) string {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeKnownHosts(t *testing.T, addr string, key ssh.PublicKey) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	line := knownhosts.Line([]string{knownhosts.Normalize(addr)}, key) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "known_hosts"), []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestRunWithStdinDeliversBytesUnchanged(t *testing.T) {
	c, records := startExecServer(t)
	content := "MAILTO=x\n5 4 * * * /usr/bin/foo \\c tail \\n \\t \\\\ 'q'\n# job @id:abc\n* * * * * bash x\n"

	if _, err := c.RunWithStdin("crontab -", content); err != nil {
		t.Fatalf("RunWithStdin: %v", err)
	}
	got := <-records
	if got.cmd != "crontab -" {
		t.Errorf("command = %q, want %q", got.cmd, "crontab -")
	}
	if got.stdin != content {
		t.Errorf("stdin changed:\n got %q\nwant %q", got.stdin, content)
	}
}

func TestUploadSendsContentOnStdin(t *testing.T) {
	c, records := startExecServer(t)
	content := "#!/bin/bash\nprintf 'a\\tb\\n'\n"

	if err := c.Upload(content, "/tmp/x/y.sh", 0o755); err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if got := <-records; got.stdin != content {
		t.Errorf("stdin changed:\n got %q\nwant %q", got.stdin, content)
	}
}
