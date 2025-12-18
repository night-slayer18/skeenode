package main

import (
	"log"
	"runtime"
)

func main() {
	log.Println("[Skeenode Executor] Node Starting...")
	
	cores := runtime.NumCPU()
	log.Printf("[Skeenode Executor] Detected %d CPU cores", cores)

	// TODO: Register with Etcd
	// TODO: Start Redis Stream Consumer
	
	select {} // Block forever
}
