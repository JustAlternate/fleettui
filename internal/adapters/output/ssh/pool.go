package ssh

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/justalternate/fleettui/internal/domain"
	"github.com/justalternate/fleettui/internal/ports/output"
)

// PooledConnection wraps an SSH client with metadata
type PooledConnection struct {
	client    output.SSHClient
	node      *domain.Node
	lastUsed  time.Time
	createdAt time.Time
	isHealthy bool
}

// BackoffState tracks retry backoff for a node
type BackoffState struct {
	consecutiveFailures int
	nextRetryTime       time.Time
	currentDelay        time.Duration
}

// ConnectionPool manages persistent SSH connections per node
type ConnectionPool struct {
	connections   map[string]*PooledConnection
	backoffs      map[string]*BackoffState
	mu            sync.RWMutex
	idleTimeout   time.Duration
	baseDelay     time.Duration
	maxDelay      time.Duration
	clientFactory func() output.SSHClient
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool() *ConnectionPool {
	return &ConnectionPool{
		connections:   make(map[string]*PooledConnection),
		backoffs:      make(map[string]*BackoffState),
		idleTimeout:   5 * time.Minute,
		baseDelay:     5 * time.Second,
		maxDelay:      5 * time.Minute,
		clientFactory: NewClient,
	}
}

// WithClientFactory overrides the client factory for testing
func (p *ConnectionPool) WithClientFactory(factory func() output.SSHClient) *ConnectionPool {
	p.clientFactory = factory
	return p
}

// Get retrieves or creates a connection for a node.
// It releases the lock during SSH connection establishment to avoid blocking
// other goroutines that are checking healthy connections.
func (p *ConnectionPool) Get(ctx context.Context, node *domain.Node) (output.SSHClient, error) {
	p.mu.Lock()

	if backoff, exists := p.backoffs[node.IP]; exists {
		if time.Now().Before(backoff.nextRetryTime) {
			p.mu.Unlock()
			return nil, fmt.Errorf("backing off, retry after %v", backoff.nextRetryTime)
		}
	}

	if conn, exists := p.connections[node.IP]; exists {
		if conn.isHealthy && time.Since(conn.lastUsed) < p.idleTimeout {
			conn.lastUsed = time.Now()
			p.mu.Unlock()
			return conn.client, nil
		}
		_ = conn.client.Disconnect()
		delete(p.connections, node.IP)
	}

	p.mu.Unlock()

	client := p.clientFactory()
	if err := client.Connect(ctx, node); err != nil {
		return nil, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if existing, exists := p.connections[node.IP]; exists {
		_ = client.Disconnect()
		existing.lastUsed = time.Now()
		return existing.client, nil
	}

	p.connections[node.IP] = &PooledConnection{
		client:    client,
		node:      node,
		lastUsed:  time.Now(),
		createdAt: time.Now(),
		isHealthy: true,
	}

	return client, nil
}

// Return marks a connection as returned to the pool (connection stays open)
func (p *ConnectionPool) Return(nodeIP string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if conn, exists := p.connections[nodeIP]; exists {
		conn.lastUsed = time.Now()
	}
}

// RecordFailure increments backoff for a node
func (p *ConnectionPool) RecordFailure(nodeIP string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	backoff, exists := p.backoffs[nodeIP]
	if !exists {
		backoff = &BackoffState{
			consecutiveFailures: 0,
			currentDelay:        p.baseDelay,
		}
		p.backoffs[nodeIP] = backoff
	}

	backoff.consecutiveFailures++
	backoff.nextRetryTime = time.Now().Add(backoff.currentDelay)

	// Exponential backoff with cap
	backoff.currentDelay *= 2
	if backoff.currentDelay > p.maxDelay {
		backoff.currentDelay = p.maxDelay
	}

	// Remove failed connection from pool
	if conn, exists := p.connections[nodeIP]; exists {
		_ = conn.client.Disconnect()
		delete(p.connections, nodeIP)
	}
}

// RecordSuccess resets backoff for a node
func (p *ConnectionPool) RecordSuccess(nodeIP string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if backoff, exists := p.backoffs[nodeIP]; exists {
		backoff.consecutiveFailures = 0
		backoff.currentDelay = p.baseDelay
		backoff.nextRetryTime = time.Time{}
	}
}

// IsInBackoff checks if a node is currently in backoff period
func (p *ConnectionPool) IsInBackoff(nodeIP string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if backoff, exists := p.backoffs[nodeIP]; exists {
		return time.Now().Before(backoff.nextRetryTime)
	}
	return false
}

// CloseAll closes all pooled connections
func (p *ConnectionPool) CloseAll() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var lastErr error
	for ip, conn := range p.connections {
		if err := conn.client.Disconnect(); err != nil {
			lastErr = err
		}
		delete(p.connections, ip)
	}

	return lastErr
}

// CleanupIdle closes connections that have been idle too long
func (p *ConnectionPool) CleanupIdle() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	for ip, conn := range p.connections {
		if now.Sub(conn.lastUsed) > p.idleTimeout {
			_ = conn.client.Disconnect()
			delete(p.connections, ip)
		}
	}
}
