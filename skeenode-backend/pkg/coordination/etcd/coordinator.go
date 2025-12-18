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
