package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	config "skeenode/configs"
	"skeenode/pkg/coordination/etcd"
	"skeenode/pkg/executor"
	"skeenode/pkg/storage/postgres"
	"skeenode/pkg/storage/redis"
)

func main() {
	cfg := config.LoadConfig()
	log.Println("[Skeenode Executor] Starting up...")

	// Create cancellable context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	// Initialize Postgres Store (needed for reporting)
	connStr := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC", 
		cfg.DBHost, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBPort)
	execStore, err := postgres.NewPostgresStore(connStr)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer execStore.Close()

	// Initialize Etcd (reuse coordination logic)
	etcdCoord, err := etcd.NewEtcdCoordinator(cfg.EtcdEndpoints, cfg.LeaderElectionTTL)
	if err != nil {
		log.Fatalf("Failed to connect to etcd: %v", err)
	}
	defer etcdCoord.Close()

	// Initialize Redis Queue
	redisAddr := fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort)
	queue, err := redis.NewRedisQueue(redisAddr)
	if err != nil {
		log.Fatalf("Failed to initialize redis queue: %v", err)
	}
	defer queue.Close()
	
	// Create Executor Core
	exec := executor.NewExecutor(cfg, etcdCoord, queue, execStore)
	
	// Run executor in goroutine so we can handle signals
	go func() {
		exec.Start(ctx)
	}()
	
	// Wait for shutdown signal
	sig := <-sigChan
	log.Printf("[Skeenode Executor] Received signal %v, initiating graceful shutdown...", sig)
	
	// Cancel context to stop executor loop
	// Note: Any in-flight job will complete due to the context check in consumeOne
	cancel()
	
	log.Println("[Skeenode Executor] Shutdown complete.")
}

