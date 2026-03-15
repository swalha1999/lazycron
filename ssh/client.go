package ssh

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

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

	hostKeyCallback := ssh.InsecureIgnoreHostKey()
	// Try known_hosts if available
	home, _ := os.UserHomeDir()
	knownHostsPath := home + "/.ssh/known_hosts"
	if _, err := os.Stat(knownHostsPath); err == nil {
		if cb, err := knownhosts.New(knownHostsPath); err == nil {
			hostKeyCallback = cb
		}
	}

	config := &ssh.ClientConfig{
		User:            c.user,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
	}

	addr := fmt.Sprintf("%s:%d", c.host, c.port)
	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
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
