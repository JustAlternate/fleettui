package ssh

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"fleettui/internal/domain"
	"fleettui/internal/ports/output"
	"golang.org/x/crypto/ssh"
)

const (
	keepaliveInterval = 5 * time.Second
	keepaliveMaxMiss  = 3
)

type Client struct {
	mu            sync.Mutex
	client        *ssh.Client
	node          *domain.Node
	lastNet       *netStats
	stopKeepalive chan struct{}
}

type netStats struct {
	rxBytes   uint64
	txBytes   uint64
	timestamp time.Time
}

func NewClient() output.SSHClient {
	return &Client{}
}

func (c *Client) Connect(ctx context.Context, node *domain.Node) error {
	// Already connected — reuse the existing session.
	c.mu.Lock()
	if c.client != nil {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	key, err := os.ReadFile(node.SSHKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read SSH key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return fmt.Errorf("failed to parse SSH key: %w", err)
	}

	config := &ssh.ClientConfig{
		User: node.User,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	addr := parseAddr(node.IP)

	// Use context-aware dialer
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", node.IP, err)
	}

	// Create SSH connection over the TCP connection
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("failed to establish SSH connection to %s: %w", node.IP, err)
	}

	client := ssh.NewClient(sshConn, chans, reqs)

	c.mu.Lock()
	c.client = client
	c.node = node

	// Start SSH-level keepalives. The Go x/crypto/ssh package has no built-in
	// ServerAliveInterval so we send "keepalive@golang.org" global requests
	// on a ticker. After keepaliveMaxMiss consecutive failures the underlying
	// connection is forcibly closed, causing any in-flight commands to fail
	// immediately rather than blocking until the idle timeout.
	c.stopKeepalive = make(chan struct{})
	stopCh := c.stopKeepalive
	c.mu.Unlock()

	go c.runKeepalive(stopCh)

	return nil
}

func (c *Client) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopKeepalive != nil {
		close(c.stopKeepalive)
		c.stopKeepalive = nil
	}
	if c.client != nil {
		err := c.client.Close()
		c.client = nil
		return err
	}
	return nil
}

// runKeepalive sends SSH-layer keepalive requests every keepaliveInterval.
// After keepaliveMaxMiss consecutive missed replies the connection is closed,
// allowing the pool to detect the dead link within ~15 seconds.
func (c *Client) runKeepalive(stop <-chan struct{}) {
	ticker := time.NewTicker(keepaliveInterval)
	defer ticker.Stop()

	missed := 0
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			c.mu.Lock()
			client := c.client
			c.mu.Unlock()

			if client == nil {
				return
			}

			_, _, err := client.SendRequest("keepalive@golang.org", true, nil)
			if err != nil {
				missed++
				if missed >= keepaliveMaxMiss {
					c.mu.Lock()
					if c.client != nil {
						_ = c.client.Close()
						c.client = nil
					}
					c.mu.Unlock()
					return
				}
			} else {
				missed = 0
			}
		}
	}
}

func (c *Client) ExecuteCommand(ctx context.Context, command string) (string, error) {
	c.mu.Lock()
	client := c.client
	c.mu.Unlock()

	if client == nil {
		return "", fmt.Errorf("not connected")
	}

	// Check context before starting
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer func() { _ = session.Close() }()

	// Execute command with context awareness
	outputCh := make(chan []byte, 1)
	errCh := make(chan error, 1)

	go func() {
		output, err := session.CombinedOutput(command)
		if err != nil {
			errCh <- err
			return
		}
		outputCh <- output
	}()

	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGTERM)
		return "", ctx.Err()
	case err := <-errCh:
		return "", fmt.Errorf("command failed: %w", err)
	case output := <-outputCh:
		return string(output), nil
	}
}

func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.client != nil
}

func parseAddr(ip string) string {
	if _, _, err := net.SplitHostPort(ip); err == nil {
		return ip
	}
	return fmt.Sprintf("%s:22", ip)
}
