package ssh

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Client represents an SSH client connection to a QNAP device
type Client struct {
	config *ssh.ClientConfig
	client *ssh.Client
	host   string
	port   int
}

// Config represents SSH connection configuration
type Config struct {
	Host     string
	Port     int
	Username string
	KeyFile  string
	Password string
	Timeout  time.Duration
}

// NewClient creates a new SSH client
func NewClient(cfg Config) (*Client, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	authMethods, err := getAuthMethods(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to get authentication methods: %w", err)
	}

	hostKeyCallback, err := getHostKeyCallback()
	if err != nil {
		return nil, fmt.Errorf("failed to get host key callback: %w", err)
	}

	sshConfig := &ssh.ClientConfig{
		User:            cfg.Username,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         cfg.Timeout,
	}

	return &Client{
		config: sshConfig,
		host:   cfg.Host,
		port:   cfg.Port,
	}, nil
}

// Connect establishes the SSH connection
func (c *Client) Connect() error {
	address := fmt.Sprintf("%s:%d", c.host, c.port)

	client, err := ssh.Dial("tcp", address, c.config)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", address, err)
	}

	c.client = client
	return nil
}

// Close closes the SSH connection
func (c *Client) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// Execute runs a command on the remote host and returns the output
func (c *Client) Execute(command string) (string, error) {
	if c.client == nil {
		return "", fmt.Errorf("not connected")
	}

	session, err := c.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer func() {
		if err := session.Close(); err != nil {
			// Session close errors are often expected (e.g., when command completes normally)
			// So we don't log this as it creates noise
		}
	}()

	output, err := session.CombinedOutput(command)
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w", err)
	}

	return string(output), nil
}

// ExecuteWithInput runs a command with input and returns the output
func (c *Client) ExecuteWithInput(command string, input io.Reader) (string, error) {
	if c.client == nil {
		return "", fmt.Errorf("not connected")
	}

	session, err := c.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer func() {
		if err := session.Close(); err != nil {
			// Session close errors are often expected (e.g., when command completes normally)
			// So we don't log this as it creates noise
		}
	}()

	session.Stdin = input
	output, err := session.CombinedOutput(command)
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w", err)
	}

	return string(output), nil
}

// IsConnected returns whether the client is connected
func (c *Client) IsConnected() bool {
	return c.client != nil
}

// TestConnection tests the SSH connection
func (c *Client) TestConnection() error {
	_, err := c.Execute("echo 'connection test'")
	return err
}

// getAuthMethods returns the authentication methods for SSH
func getAuthMethods(cfg Config) ([]ssh.AuthMethod, error) {
	var authMethods []ssh.AuthMethod

	// Try SSH agent first
	if agentAuth := trySSHAgent(); agentAuth != nil {
		authMethods = append(authMethods, agentAuth)
	}

	// Try private key file
	if cfg.KeyFile != "" {
		keyAuth, err := tryKeyFile(cfg.KeyFile)
		if err != nil {
			return nil, err
		}
		if keyAuth != nil {
			authMethods = append(authMethods, keyAuth)
		}
	}

	// Try default key files
	defaultKeys := []string{"id_rsa", "id_ed25519", "id_ecdsa"}
	homeDir, err := os.UserHomeDir()
	if err == nil {
		for _, keyName := range defaultKeys {
			keyPath := filepath.Join(homeDir, ".ssh", keyName)
			if keyAuth, err := tryKeyFile(keyPath); err == nil && keyAuth != nil {
				authMethods = append(authMethods, keyAuth)
			}
		}
	}

	// Try password authentication
	if cfg.Password != "" {
		authMethods = append(authMethods, ssh.Password(cfg.Password))
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no authentication methods available")
	}

	return authMethods, nil
}

// trySSHAgent attempts to use SSH agent for authentication
func trySSHAgent() ssh.AuthMethod {
	if sshAgent, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK")); err == nil {
		return ssh.PublicKeysCallback(agent.NewClient(sshAgent).Signers)
	}
	return nil
}

// tryKeyFile attempts to use a private key file for authentication
func tryKeyFile(keyPath string) (ssh.AuthMethod, error) {
	if keyPath == "" {
		return nil, nil
	}

	// Expand tilde
	if strings.HasPrefix(keyPath, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		keyPath = filepath.Join(homeDir, keyPath[2:])
	}

	// Check if file exists
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		return nil, nil // File doesn't exist, not an error
	}

	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file %s: %w", keyPath, err)
	}

	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key %s: %w", keyPath, err)
	}

	return ssh.PublicKeys(signer), nil
}

// getHostKeyCallback returns the host key callback for SSH
func getHostKeyCallback() (ssh.HostKeyCallback, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to insecure if we can't get home directory
		return ssh.InsecureIgnoreHostKey(), nil
	}

	knownHostsFile := filepath.Join(homeDir, ".ssh", "known_hosts")

	// Check if known_hosts file exists
	if _, err := os.Stat(knownHostsFile); os.IsNotExist(err) {
		// If known_hosts doesn't exist, use insecure callback
		// In a production environment, you might want to create the file
		// or prompt the user to verify the host key
		return ssh.InsecureIgnoreHostKey(), nil
	}

	callback, err := knownhosts.New(knownHostsFile)
	if err != nil {
		// Fallback to insecure if we can't parse known_hosts
		return ssh.InsecureIgnoreHostKey(), nil
	}

	return callback, nil
}
