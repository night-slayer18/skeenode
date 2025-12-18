package etcd

import (
	"context"
	"fmt"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"

	"skeenode/pkg/coordination"
)

type EtcdCoordinator struct {
	client  *clientv3.Client
	session *concurrency.Session
}

func NewEtcdCoordinator(endpoints []string, ttl int) (*EtcdCoordinator, error) {
	// Create the raw etcd client
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to etcd: %w", err)
	}

	// Create a concurrency session (keeps lease alive via heartbeats)
	sess, err := concurrency.NewSession(cli, concurrency.WithTTL(ttl))
	if err != nil {
		cli.Close()
		return nil, fmt.Errorf("failed to create concurrency session: %w", err)
	}

	return &EtcdCoordinator{
		client:  cli,
		session: sess,
	}, nil
}

func (c *EtcdCoordinator) Close() error {
	if c.session != nil {
		c.session.Close()
	}
	return c.client.Close()
}

func (c *EtcdCoordinator) NewElection(name string) coordination.Election {
	// Use the etcd concurrency/election package
	e := concurrency.NewElection(c.session, "/elections/"+name)
	return &EtcdElection{election: e}
}

// EtcdElection wraps the etcd concurrency.Election struct
type EtcdElection struct {
	election *concurrency.Election
}

func (e *EtcdElection) Campaign(ctx context.Context, value string) error {
	return e.election.Campaign(ctx, value)
}

func (e *EtcdElection) Resign(ctx context.Context) error {
	return e.election.Resign(ctx)
}

func (e *EtcdElection) Leader(ctx context.Context) (string, error) {
	resp, err := e.election.Leader(ctx)
	if err != nil {
		return "", err
	}
	return string(resp.Kvs[0].Value), nil
}

func (c *EtcdCoordinator) RegisterNode(ctx context.Context, nodeID string, ttl int) error {
	// Create a lease
	resp, err := c.client.Grant(ctx, int64(ttl))
	if err != nil {
		return fmt.Errorf("failed to grant lease: %w", err)
	}

	key := fmt.Sprintf("/nodes/%s", nodeID)
	// Put key with lease
	_, err = c.client.Put(ctx, key, "ONLINE", clientv3.WithLease(resp.ID))
	if err != nil {
		return fmt.Errorf("failed to put node key: %w", err)
	}
	
	// KeepAlive to refresh the lease automatically
	// Note: For a strict heartbeat loop called periodically, we might not need KeepAlive
	// if the caller calls RegisterNode repeatedly.
	// But using KeepAlive is cleaner/more robust for a long-running process.
	// However, to match the "Heartbeat Loop" in Executor.Start, we can just purely rely on repeated Puts/KeepAlives.
	// Let's use KeepAliveOnce for the simple repeated call pattern or start a KeepAlive channel.
	
	// Implementation choice: Since Executor.Start has a ticker, we can simple use Put with Lease repeatedly,
	// OR use a long-lived KeepAlive.
	// Let's use the robust KeepAlive channel method here attached to the session if possible, 
	// but simpler for now matches the "Heartbeat" concept:
	// The ticker in Executor calls this. So we just need to refresh the lease or re-put.
	
	// Actually, simpler approach for "Heartbeat Loop" architecture:
	// The loop simply calls this function every X seconds.
	// So we just Put with a short lease.
	return nil
}

func (c *EtcdCoordinator) GetActiveNodes(ctx context.Context) ([]string, error) {
	// List all keys under /nodes/
	resp, err := c.client.Get(ctx, "/nodes/", clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	var nodes []string
	for _, kv := range resp.Kvs {
		// Key format: /nodes/{node_id}
		// Extract node_id
		key := string(kv.Key)
		// Assuming prefix length is 7 ("/nodes/")
		if len(key) > 7 {
			nodes = append(nodes, key[7:])
		}
	}
	return nodes, nil
}
