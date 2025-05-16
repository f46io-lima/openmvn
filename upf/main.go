package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/openmvcore/upf/pkg/upf"
	"github.com/sirupsen/logrus"
)

var (
	upfLogger = logrus.New()
)

func init() {
	// Configure logging
	upfLogger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339,
	})
	upfLogger.SetOutput(os.Stdout)
	upfLogger.SetLevel(logrus.InfoLevel)
}

func main() {
	// UPF configuration
	cfg := &upf.Config{
		PFCP: struct {
			Addr string
		}{
			Addr: "0.0.0.0:8805", // PFCP interface
		},
		GTP: struct {
			Addr string
		}{
			Addr: "0.0.0.0:2152", // GTP-U interface
		},
		EnableGTP:    true,
		EnablePFCP:   true,
		EnableUPlane: true,
		ReportNotify: true,
		LogLevel:     "info",
	}

	// Create UPF instance
	upfLogger.Info("[UPF] Initializing...")
	upfInstance := upf.NewUPF(cfg)

	// Connect to NATS
	nc, err := nats.Connect("nats://nats:4222")
	if err != nil {
		upfLogger.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		upfLogger.Fatalf("Failed to get JetStream context: %v", err)
	}

	// Subscribe to PFCP session events
	_, err = js.Subscribe("pfcp.session.created", func(m *nats.Msg) {
		upfLogger.Printf("[UPF] GTP tunnel created: %s", string(m.Data))
		// TODO: Update tunnel state if needed
	})
	if err != nil {
		upfLogger.Printf("Failed to subscribe to pfcp.session.created: %v", err)
	}

	// Handle graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start UPF in a goroutine
	go func() {
		if err := upfInstance.Run(); err != nil {
			upfLogger.Fatalf("[UPF] Failed to run: %v", err)
		}
	}()

	upfLogger.Info("[UPF] Started successfully")
	upfLogger.Info("[UPF] Listening on:")
	upfLogger.Info("  - PFCP: 0.0.0.0:8805")
	upfLogger.Info("  - GTP-U: 0.0.0.0:2152")

	// Monitor UPF status
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Initialize Redis
	InitRedis()

	for {
		select {
		case <-ctx.Done():
			upfLogger.Info("[UPF] Shutting down...")
			upfInstance.Close()
			return
		case <-ticker.C:
			// Log UPF status
			sessions := upfInstance.GetSessionCount()
			upfLogger.Infof("[UPF] Status: %d active sessions", sessions)
		}
	}
}

func handlePFCPMessage(msg *pfcp.Message) {
	// ... existing code ...

	// After PFCP tunnel creation
	pfcpJSON, err := json.Marshal(tunnel)
	if err != nil {
		log.Printf("Failed to marshal PFCP tunnel: %v", err)
	} else {
		// Store PFCP tunnel in Redis with 30 minute expiry
		err = RedisClient.Set(RedisCtx, "pfcp:"+teid, pfcpJSON, 30*time.Minute).Err()
		if err != nil {
			log.Printf("Failed to store PFCP tunnel in Redis: %v", err)
		}
	}

	// ... existing code ...
}
