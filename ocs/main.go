package main

import (
	"log"
	"time"

	"github.com/nats-io/nats.go"
)

func main() {
	// Connect to NATS
	nc, err := nats.Connect("nats://nats:4222")
	if err != nil {
		log.Fatalf("❌ Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	// Create publisher
	publisher := NewPublisher(nc)
	if publisher == nil {
		log.Fatal("❌ Failed to create publisher")
	}

	// Test publish
	log.Println("🚀 Starting OCS service...")
	for {
		publisher.PublishQuotaDeducted("123456789012345", 100, 900)
		log.Println("✅ Published quota deduction event")
		time.Sleep(5 * time.Second)
	}
}
