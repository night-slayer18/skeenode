package main

import (
	"log"
	"time"
)

func main() {
	log.Println("[Skeenode Scheduler] Starting up...")
	
	// TODO: Initialize Etcd Connection
	// TODO: Start Leader Election
	
	ticker := time.NewTicker(10 * time.Second)
	for t := range ticker.C {
		log.Printf("[Skeenode Scheduler] Heartbeat at %s", t.Format(time.RFC3339))
	}
}
