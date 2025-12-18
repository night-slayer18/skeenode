package main

import (
	"context"
	"fmt"
	"log"
	"os"

	config "skeenode/configs"
	"skeenode/pkg/coordination/etcd"
	"skeenode/pkg/scheduler"
	"skeenode/pkg/storage/postgres"
	"skeenode/pkg/storage/redis"

	"github.com/google/uuid"
)

func main() {
	cfg := config.LoadConfig()
	log.Println("[Skeenode Scheduler] Starting up...")
	
	ctx := context.Background()
	
	// Initialize Postgres Store (GORM)
	connStr := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC", 
		cfg.DBHost, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBPort)
		
	store, err := postgres.NewPostgresStore(connStr)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()
	log.Println("[Skeenode Scheduler] Postgres connected & schema initialized.")

	// Initialize Redis Queue
	redisAddr := fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort)
	queue, err := redis.NewRedisQueue(redisAddr)
	if err != nil {
		log.Fatalf("Failed to initialize redis queue: %v", err)
	}
	defer queue.Close()
	log.Println("[Skeenode Scheduler] Redis connected & queue initialized.")
	
	// Initialize Etcd Coordinator
	etcdCoord, err := etcd.NewEtcdCoordinator(cfg.EtcdEndpoints, cfg.LeaderElectionTTL)
	if err != nil {
		log.Fatalf("Failed to connect to etcd: %v", err)
	}
	defer etcdCoord.Close()
	log.Println("[Skeenode Scheduler] Connected to Etcd.")

	// Start Leader Election
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "scheduler-" + uuid.New().String()
	}
	election := etcdCoord.NewElection("skeenode-leader")
	
	log.Printf("Follower: requesting leadership as %s...", hostname)
	if err := election.Campaign(ctx, hostname); err != nil {
		log.Fatalf("Election campaign failed: %v", err)
	}
	log.Println("Leader: I am the captain now.")
	
	// Create and Run Scheduler Core
	core := scheduler.NewCore(cfg, store, store, queue, etcdCoord)
	log.Println("[Skeenode Scheduler] Starting main work loop...")
	
	// Blocking call
	core.Run(ctx, election)
}
