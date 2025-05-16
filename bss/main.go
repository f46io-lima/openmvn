package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
)

var log = logrus.New()

type Subscriber struct {
	IMSI    string    `json:"imsi"`
	Token   string    `json:"token"`
	Balance int       `json:"balance"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
}

type TopUpRequest struct {
	IMSI   string `json:"imsi"`
	Amount int    `json:"amount"`
}

type TopUpResponse struct {
	IMSI    string `json:"imsi"`
	Balance int    `json:"balance"`
	Message string `json:"message,omitempty"`
}

var (
	simDB     = make(map[string]*Subscriber)
	simDBLock sync.RWMutex
)

func main() {
	// Configure logging
	log.SetFormatter(&logrus.JSONFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(logrus.InfoLevel)

	// Initialize router
	r := mux.NewRouter()

	// Add middleware
	r.Use(loggingMiddleware)
	r.Use(recoveryMiddleware)

	// Register routes
	r.HandleFunc("/register", registerHandler).Methods("POST")
	r.HandleFunc("/topup", topUpHandler).Methods("POST")
	r.HandleFunc("/subscriber/{imsi}", getSubscriberHandler).Methods("GET")
	r.HandleFunc("/health", healthHandler).Methods("GET")

	// Connect to NATS
	nc, err := nats.Connect("nats://nats:4222")
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		log.Fatalf("Failed to get JetStream context: %v", err)
	}

	// Subscribe to UE registration events for logging
	_, err = js.Subscribe("ue.registered", func(m *nats.Msg) {
		log.Printf("[BSS] UE registered event: %s", string(m.Data))
		// TODO: Update billing state if needed
	})
	if err != nil {
		log.Printf("Failed to subscribe to ue.registered: %v", err)
	}

	// Create server with timeouts
	srv := &http.Server{
		Addr:         ":8083",
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Info("Starting BSS server on :8083")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Graceful shutdown
	log.Info("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Info("Server exited properly")
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	var sub Subscriber
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	// Validate IMSI format (basic check)
	if len(sub.IMSI) < 10 {
		http.Error(w, "invalid IMSI format", http.StatusBadRequest)
		return
	}

	now := time.Now()
	sub.Created = now
	sub.Updated = now

	simDBLock.Lock()
	defer simDBLock.Unlock()

	if _, exists := simDB[sub.IMSI]; exists {
		http.Error(w, "subscriber already registered", http.StatusConflict)
		return
	}

	simDB[sub.IMSI] = &sub
	log.WithFields(logrus.Fields{
		"imsi":    sub.IMSI,
		"balance": sub.Balance,
	}).Info("New subscriber registered")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(sub)
}

func topUpHandler(w http.ResponseWriter, r *http.Request) {
	var req TopUpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if req.Amount <= 0 {
		http.Error(w, "invalid amount", http.StatusBadRequest)
		return
	}

	simDBLock.Lock()
	defer simDBLock.Unlock()

	sub, exists := simDB[req.IMSI]
	if !exists {
		http.Error(w, "subscriber not found", http.StatusNotFound)
		return
	}

	sub.Balance += req.Amount
	sub.Updated = time.Now()

	resp := TopUpResponse{
		IMSI:    sub.IMSI,
		Balance: sub.Balance,
		Message: "Top-up successful",
	}

	log.WithFields(logrus.Fields{
		"imsi":        sub.IMSI,
		"amount":      req.Amount,
		"new_balance": sub.Balance,
	}).Info("Balance top-up completed")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func getSubscriberHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	imsi := vars["imsi"]

	simDBLock.RLock()
	defer simDBLock.RUnlock()

	sub, exists := simDB[imsi]
	if !exists {
		http.Error(w, "subscriber not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sub)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.WithFields(logrus.Fields{
			"method":   r.Method,
			"path":     r.RequestURI,
			"duration": time.Since(start),
		}).Info("Request processed")
	})
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.WithField("error", err).Error("Panic recovered")
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
