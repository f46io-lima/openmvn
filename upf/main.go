package main

import (
	"log"
	"time"
)

func main() {
	log.Println("📡 UPF Booting...")

	// Step 1: Connect to Redis
	InitRedis()

	// Step 2: Start PFCP listener (stub)
	StartPFCPAgent()

	// Step 3: Setup GTP tunnels (stub)
	SetupTunnels()

	// Simulate running state
	for {
		log.Println("💡 UPF active and waiting for PFCP/TEID setup...")
		time.Sleep(15 * time.Second)
	}
}
