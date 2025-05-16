package main

import (
	"encoding/asn1"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ishidawataru/sctp"
)

// NAS represents a Non-Access Stratum message
type NAS struct {
	IMSI string
	Auth string
}

// NGAPMessage represents a simplified NGAP message
type NGAPMessage struct {
	MessageType int
	UEID       uint64
	NASMsg     NAS
	Timestamp  time.Time
}

// NGAPResponse represents the AMF's response
type NGAPResponse struct {
	MessageType int
	Success     bool
	Reason      string
	Timestamp   time.Time
}

func main() {
	// Parse command line flags
	amfAddr := flag.String("amf", "localhost", "AMF address")
	imsi := flag.String("imsi", "001010123456789", "IMSI to use")
	auth := flag.String("auth", "secret123", "Authentication string")
	flag.Parse()

	// Connect to AMF
	peer := &sctp.SCTPAddr{
		IPAddrs: []net.IPAddr{{IP: net.ParseIP(*amfAddr)}},
		Port:    38412,
	}

	conn, err := sctp.DialSCTP("sctp", nil, peer)
	if err != nil {
		log.Fatalf("[gNB] Dial failed: %v", err)
	}
	defer conn.Close()

	log.Printf("[gNB] Connected to AMF at %s", *amfAddr)

	// Handle shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("[gNB] Shutting down...")
		conn.Close()
		os.Exit(0)
	}()

	// Generate random UE ID
	rand.Seed(time.Now().UnixNano())
	ueid := rand.Uint64()

	// Send registration message
	msg := NGAPMessage{
		MessageType: 1, // Registration request
		UEID:       ueid,
		NASMsg: NAS{
			IMSI: *imsi,
			Auth: *auth,
		},
		Timestamp: time.Now(),
	}

	payload, err := asn1.Marshal(msg)
	if err != nil {
		log.Fatalf("[gNB] ASN.1 marshal failed: %v", err)
	}

	n, err := conn.Write(payload)
	if err != nil {
		log.Fatalf("[gNB] Write error: %v", err)
	}
	log.Printf("[gNB] Sent registration: UEID=%d IMSI=%s (%d bytes)", ueid, *imsi, n)

	// Wait for response
	buf := make([]byte, 1024)
	n, err = conn.Read(buf)
	if err != nil {
		log.Fatalf("[gNB] Read error: %v", err)
	}

	var resp NGAPResponse
	_, err = asn1.Unmarshal(buf[:n], &resp)
	if err != nil {
		log.Fatalf("[gNB] ASN.1 unmarshal failed: %v", err)
	}

	if resp.Success {
		log.Printf("[gNB] Registration successful ✅: %s", resp.Reason)
	} else {
		log.Printf("[gNB] Registration failed ❌: %s", resp.Reason)
	}

	// Keep connection alive
	select {}
} 