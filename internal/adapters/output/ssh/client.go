package ssh

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"fleettui/internal/domain"
	"fleettui/internal/ports/output"
	"golang.org/x/crypto/ssh"
)

type Client struct {
	client  *ssh.Client
	node    *domain.Node
	lastNet *netStats
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
		conn.Close()
		return fmt.Errorf("failed to establish SSH connection to %s: %w", node.IP, err)
	}

	client := ssh.NewClient(sshConn, chans, reqs)

	c.client = client
	c.node = node
	return nil
}

func (c *Client) Disconnect() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

func (c *Client) ExecuteCommand(ctx context.Context, command string) (string, error) {
	if c.client == nil {
		return "", fmt.Errorf("not connected")
	}

	// Check context before starting
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	session, err := c.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

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
		session.Signal(ssh.SIGTERM)
		return "", ctx.Err()
	case err := <-errCh:
		return "", fmt.Errorf("command failed: %w", err)
	case output := <-outputCh:
		return string(output), nil
	}
}

func (c *Client) IsConnected() bool {
	return c.client != nil
}

func parseAddr(ip string) string {
	if _, _, err := net.SplitHostPort(ip); err == nil {
		return ip
	}
	return fmt.Sprintf("%s:22", ip)
}
