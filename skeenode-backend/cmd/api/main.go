package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	config "skeenode/configs"
	"skeenode/pkg/api"
	"skeenode/pkg/coordination/etcd"
	"skeenode/pkg/storage/postgres"
	"skeenode/pkg/storage/redis"
)

func main() {
	cfg := config.LoadConfig()
	log.Println("[Skeenode API] Starting up...")

	// Create cancellable context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Initialize Postgres Store
	connStr := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		cfg.DBHost, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBPort)

	store, err := postgres.NewPostgresStore(connStr)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()
	log.Println("[Skeenode API] Postgres connected.")

	// Initialize Etcd Coordinator
	etcdCoord, err := etcd.NewEtcdCoordinator(cfg.EtcdEndpoints, cfg.LeaderElectionTTL)
	if err != nil {
		log.Fatalf("Failed to connect to etcd: %v", err)
	}
	defer etcdCoord.Close()
	log.Println("[Skeenode API] Etcd connected.")

	// Initialize Redis Queue
	redisAddr := fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort)
	queue, err := redis.NewRedisQueue(redisAddr)
	if err != nil {
		log.Fatalf("Failed to initialize redis queue: %v", err)
	}
	defer queue.Close()
	log.Println("[Skeenode API] Redis connected.")

	// Create API Server
	apiPort := cfg.APIPort
	if apiPort == "" {
		apiPort = "8080"
	}

	server := api.NewServer(api.Config{
		Port:        apiPort,
		JobStore:    store,
		ExecStore:   store,
		DepStore:    store,
		Queue:       queue,
		Coordinator: etcdCoord,
	})

	// Run API server in goroutine
	go func() {
		if err := server.Start(); err != nil {
			log.Printf("[Skeenode API] Server error: %v", err)
		}
	}()

	log.Printf("[Skeenode API] Server started on port %s", apiPort)

	// Wait for shutdown signal
	sig := <-sigChan
	log.Printf("[Skeenode API] Received signal %v, initiating graceful shutdown...", sig)

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("[Skeenode API] Shutdown error: %v", err)
	}

	cancel()
	log.Println("[Skeenode API] Shutdown complete.")
}
