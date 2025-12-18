package main

import (
	"context"
	"log"

	"fmt"
	config "skeenode/configs"
	"skeenode/pkg/coordination/etcd"
	"skeenode/pkg/executor"

	"skeenode/pkg/storage/postgres"
	"skeenode/pkg/storage/redis"
)

func main() {
	cfg := config.LoadConfig()
	log.Println("[Skeenode Executor] Starting up...")

	ctx := context.Background()
	
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
	
	// Start Executor Core
	exec := executor.NewExecutor(cfg, etcdCoord, queue, execStore)
	exec.Start(ctx)
}
