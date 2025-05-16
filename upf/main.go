package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

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
