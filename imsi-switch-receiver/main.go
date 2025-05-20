package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/nats-io/nats.go"
)

// SwitchDecision represents a pending IMSI switch decision
type SwitchDecision struct {
	IMSI     string    `json:"imsi"`
	OldIMSI  string    `json:"old_imsi"`
	DeviceID string    `json:"device_id"`
	Reason   string    `json:"reason"`
	Time     time.Time `json:"timestamp"`
	Status   string    `json:"status"` // "pending", "accepted", "rejected"
}

var (
	// In-memory store of pending switch decisions
	pendingSwitches = make(map[string]*SwitchDecision)
	mu              sync.RWMutex
)

func main() {
	log.Println("📱 IMSI Switch Receiver Starting...")

	// Connect to NATS
	nc, err := nats.Connect("nats://nats:4222")
	if err != nil {
		log.Fatalf("❌ NATS connection failed: %v", err)
	}
	defer nc.Close()

	// Subscribe to IMSI switch events
	js, err := nc.JetStream()
	if err != nil {
		log.Fatalf("❌ JetStream context failed: %v", err)
	}

	// Create a stream if it doesn't exist
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     "IMSI_SWITCHES",
		Subjects: []string{"imsi.switch"},
	})
	if err != nil {
		log.Printf("ℹ️ Stream might already exist: %v", err)
	}

	// Subscribe to switch events
	_, err = js.Subscribe("imsi.switch", func(m *nats.Msg) {
		var event SwitchDecision
		if err := json.Unmarshal(m.Data, &event); err != nil {
			log.Printf("❌ Failed to unmarshal switch event: %v", err)
			return
		}

		// Store the switch decision
		mu.Lock()
		pendingSwitches[event.DeviceID] = &event
		mu.Unlock()

		log.Printf("📥 Received switch decision for device %s: %s -> %s",
			event.DeviceID, event.OldIMSI, event.IMSI)
	})
	if err != nil {
		log.Fatalf("❌ Failed to subscribe: %v", err)
	}

	// Initialize router
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Routes
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	// Get pending switch for a device
	r.Get("/switches/{deviceID}", func(w http.ResponseWriter, r *http.Request) {
		deviceID := chi.URLParam(r, "deviceID")

		mu.RLock()
		decision, exists := pendingSwitches[deviceID]
		mu.RUnlock()

		if !exists {
			http.Error(w, "No pending switch found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(decision)
	})

	// Accept or reject a switch
	r.Post("/switches/{deviceID}/respond", func(w http.ResponseWriter, r *http.Request) {
		deviceID := chi.URLParam(r, "deviceID")

		var response struct {
			Status string `json:"status"` // "accepted" or "rejected"
		}
		if err := json.NewDecoder(r.Body).Decode(&response); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		mu.Lock()
		decision, exists := pendingSwitches[deviceID]
		if !exists {
			mu.Unlock()
			http.Error(w, "No pending switch found", http.StatusNotFound)
			return
		}

		decision.Status = response.Status
		if response.Status == "accepted" || response.Status == "rejected" {
			delete(pendingSwitches, deviceID)
		}
		mu.Unlock()

		log.Printf("📤 Device %s %s switch to IMSI %s",
			deviceID, response.Status, decision.IMSI)

		w.WriteHeader(http.StatusOK)
	})

	// Start server
	log.Println("🌐 Server starting on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatalf("❌ Server failed: %v", err)
	}
}
