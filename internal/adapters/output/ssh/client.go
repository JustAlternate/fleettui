package ssh

import (
	"context"
	"fmt"
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
		Timeout:         10 * time.Second,
	}

	addr := fmt.Sprintf("%s:22", node.IP)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", node.IP, err)
	}

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

	session, err := c.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w", err)
	}

	return string(output), nil
}

func (c *Client) IsConnected() bool {
	return c.client != nil
}
