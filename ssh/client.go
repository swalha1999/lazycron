package ssh

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// ErrHostKeyVerification is returned when known_hosts is missing or unreadable.
var ErrHostKeyVerification = errors.New("host key verification failed")

// AuthError is returned when SSH authentication fails, signaling that a
// password may be needed.
type AuthError struct {
	Err error
}

// Client wraps an SSH connection with convenience methods.
type Client struct {
	host     string
	port     int
	user     string
	password string
	keyPath  string
	useAgent bool

	mu   sync.Mutex
	conn *ssh.Client
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("authentication failed: %v", e.Err)
}

func (e *AuthError) Unwrap() error {
	return e.Err
}

// NewClient creates a new SSH client (does not connect immediately).
func NewClient(host string, port int, user, password, keyPath string, useAgent bool) *Client {
	if port == 0 {
		port = 22
	}
	return &Client{
		host:     host,
		port:     port,
		user:     user,
		password: password,
		keyPath:  keyPath,
		useAgent: useAgent,
	}
}

// SetPassword sets the password for authentication at runtime (not persisted).
func (c *Client) SetPassword(pw string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.password = pw
}

// Connect establishes the SSH connection.
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return nil
	}

	var authMethods []ssh.AuthMethod

	// Try ssh-agent first if requested
	if c.useAgent {
		if method := agentAuth(); method != nil {
			authMethods = append(authMethods, method)
		}
	}

	// Try key file
	if c.keyPath != "" {
		if method, err := keyFileAuth(c.keyPath); err == nil {
			authMethods = append(authMethods, method)
		}
	}

	// Fallback: try default key locations
	if len(authMethods) == 0 {
		for _, name := range []string{"id_ed25519", "id_rsa", "id_ecdsa"} {
			home, _ := os.UserHomeDir()
			path := home + "/.ssh/" + name
			if method, err := keyFileAuth(path); err == nil {
				authMethods = append(authMethods, method)
			}
		}
	}

	// Also try agent as last resort
	if !c.useAgent {
		if method := agentAuth(); method != nil {
			authMethods = append(authMethods, method)
		}
	}

	// Try password auth
	if c.password != "" {
		authMethods = append(authMethods, ssh.Password(c.password))
		authMethods = append(authMethods, ssh.KeyboardInteractive(
			func(user, instruction string, questions []string, echos []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range answers {
					answers[i] = c.password
				}
				return answers, nil
			},
		))
	}

	if len(authMethods) == 0 {
		return fmt.Errorf("no SSH auth methods available")
	}

	home, _ := os.UserHomeDir()
	knownHostsPath := home + "/.ssh/known_hosts"
	if _, err := os.Stat(knownHostsPath); err != nil {
		return fmt.Errorf("%w: %s not found — for your security, run \"ssh %s@%s\" first to verify and save the server's host key",
			ErrHostKeyVerification, knownHostsPath, c.user, c.host)
	}
	hostKeyCallback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return fmt.Errorf("%w: failed to parse %s — %v",
			ErrHostKeyVerification, knownHostsPath, err)
	}

	config := &ssh.ClientConfig{
		User:            c.user,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
	}

	addr := fmt.Sprintf("%s:%d", c.host, c.port)
	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		if strings.Contains(err.Error(), "unable to authenticate") {
			return &AuthError{Err: err}
		}
		if strings.Contains(err.Error(), "key is unknown") {
			return fmt.Errorf("%w: server %s not in known_hosts — run \"ssh %s@%s\" first to verify and save its host key",
				ErrHostKeyVerification, c.host, c.user, c.host)
		}
		return fmt.Errorf("ssh dial %s: %w", addr, err)
	}

	c.conn = conn
	return nil
}

// Run executes a command on the remote server.
func (c *Client) Run(cmd string) (string, error) {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		if err := c.Connect(); err != nil {
			return "", err
		}
		c.mu.Lock()
		conn = c.conn
		c.mu.Unlock()
	}

	session, err := conn.NewSession()
	if err != nil {
		// Connection may have dropped — try reconnect once
		c.mu.Lock()
		c.conn = nil
		c.mu.Unlock()
		if err := c.Connect(); err != nil {
			return "", err
		}
		c.mu.Lock()
		conn = c.conn
		c.mu.Unlock()
		session, err = conn.NewSession()
		if err != nil {
			return "", fmt.Errorf("ssh session: %w", err)
		}
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	err = session.Run(cmd)
	output := strings.TrimSpace(stdout.String())
	if err != nil && output == "" {
		output = strings.TrimSpace(stderr.String())
	}
	return output, err
}

// Upload writes content to a remote file path.
func (c *Client) Upload(content, path string, mode os.FileMode) error {
	// Use cat to write content via stdin
	escapedPath := strings.ReplaceAll(path, "'", "'\\''")
	cmd := fmt.Sprintf("mkdir -p \"$(dirname '%s')\" && cat > '%s' && chmod %o '%s'",
		escapedPath, escapedPath, mode, escapedPath)

	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
		c.mu.Lock()
		conn = c.conn
		c.mu.Unlock()
	}

	session, err := conn.NewSession()
	if err != nil {
		return fmt.Errorf("ssh session: %w", err)
	}
	defer session.Close()

	session.Stdin = strings.NewReader(content)
	return session.Run(cmd)
}

// UploadDirectory tars localDir locally, streams the tarball over the SSH
// connection's stdin, and untars into remoteDir. excludes are tar --exclude
// patterns relative to localDir's root (e.g. "./.env"). The remote directory
// is created if missing.
//
// Implementation note: we use stdin piping rather than scp/sftp to match the
// rest of this client's transport pattern, and to avoid a binary dependency
// on rsync. tar's --exclude form is portable across BSD and GNU tar.
func (c *Client) UploadDirectory(localDir, remoteDir string, excludes []string) error {
	if _, err := c.Run("mkdir -p " + shellQuoteSingle(remoteDir)); err != nil {
		return fmt.Errorf("create remote dir: %w", err)
	}

	args := []string{"-czf", "-", "-C", localDir}
	for _, e := range excludes {
		args = append(args, "--exclude="+e)
	}
	args = append(args, ".")
	tarCmd := exec.Command("tar", args...)
	stdout, err := tarCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("local tar stdout pipe: %w", err)
	}
	var localErr bytes.Buffer
	tarCmd.Stderr = &localErr
	if err := tarCmd.Start(); err != nil {
		return fmt.Errorf("start local tar: %w", err)
	}

	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	if conn == nil {
		if err := c.Connect(); err != nil {
			tarCmd.Process.Kill()
			return err
		}
		c.mu.Lock()
		conn = c.conn
		c.mu.Unlock()
	}

	session, err := conn.NewSession()
	if err != nil {
		tarCmd.Process.Kill()
		return fmt.Errorf("ssh session: %w", err)
	}
	defer session.Close()

	session.Stdin = stdout
	var remoteErr bytes.Buffer
	session.Stderr = &remoteErr
	session.Stdout = io.Discard

	runErr := session.Run("tar -xzf - -C " + shellQuoteSingle(remoteDir))
	tarWaitErr := tarCmd.Wait()

	if tarWaitErr != nil {
		return fmt.Errorf("local tar: %w (stderr: %s)", tarWaitErr, strings.TrimSpace(localErr.String()))
	}
	if runErr != nil {
		return fmt.Errorf("remote untar: %w (stderr: %s)", runErr, strings.TrimSpace(remoteErr.String()))
	}
	return nil
}

// HasCommand returns true if `command -v <name>` exits zero on the remote.
// Used to verify presence of binaries like docker/node/npx before sync.
func (c *Client) HasCommand(name string) bool {
	_, err := c.Run("command -v " + shellQuoteSingle(name) + " >/dev/null 2>&1")
	return err == nil
}

// FileExists reports whether a regular file exists at path on the remote.
// Returns (false, nil) for missing files and (false, err) for transport errors.
func (c *Client) FileExists(path string) (bool, error) {
	_, err := c.Run("test -f " + shellQuoteSingle(path))
	if err == nil {
		return true, nil
	}
	if exitErr, ok := err.(*ssh.ExitError); ok && exitErr.ExitStatus() == 1 {
		return false, nil
	}
	return false, err
}

// shellQuoteSingle wraps s in single quotes, escaping any embedded singles.
func shellQuoteSingle(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// ReadFile reads a file from the remote server.
func (c *Client) ReadFile(path string) ([]byte, error) {
	output, err := c.Run(fmt.Sprintf("cat '%s'", strings.ReplaceAll(path, "'", "'\\''")))
	if err != nil {
		return nil, err
	}
	return []byte(output), nil
}

// ListFiles lists files in a remote directory matching a pattern.
func (c *Client) ListFiles(dir, pattern string) ([]string, error) {
	cmd := fmt.Sprintf("ls -1 '%s'/%s 2>/dev/null || true",
		strings.ReplaceAll(dir, "'", "'\\''"), pattern)
	output, err := c.Run(cmd)
	if err != nil {
		return nil, err
	}
	if output == "" {
		return nil, nil
	}
	return strings.Split(output, "\n"), nil
}

// IsConnected returns whether an SSH connection is active.
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn != nil
}

// Close terminates the SSH connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

func keyFileAuth(path string) (ssh.AuthMethod, error) {
	key, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeys(signer), nil
}

func agentAuth() ssh.AuthMethod {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil
	}
	agentClient := agent.NewClient(conn)
	return ssh.PublicKeysCallback(agentClient.Signers)
}
