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

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
)

var log = logrus.New()

type QuotaRequest struct {
	IMSI string `json:"imsi"`
	MB   int    `json:"mb"`
}

type QuotaResponse struct {
	Approved bool   `json:"approved"`
	Reason   string `json:"reason,omitempty"`
	Balance  int    `json:"balance,omitempty"`
}

type SubscriberBalance struct {
	IMSI     string    `json:"imsi"`
	Balance  int       `json:"balance"`
	Updated  time.Time `json:"updated"`
	LastUsed time.Time `json:"last_used,omitempty"`
}

var (
	balanceStore = map[string]*SubscriberBalance{
		"001010123456789": {
			IMSI:    "001010123456789",
			Balance: 100, // 100MB
			Updated: time.Now(),
		},
	}
	balanceLock sync.RWMutex
)

// Initialize Redis
var RedisClient *redis.Client
var RedisCtx = context.Background()

func InitRedis() {
	RedisClient = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
}

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
	r.HandleFunc("/quota", quotaHandler).Methods("POST")
	r.HandleFunc("/balance/{imsi}", getBalanceHandler).Methods("GET")
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

	publisher := NewPublisher(nc)

	// Subscribe to PFCP session events
	_, err = js.Subscribe("pfcp.session.created", func(m *nats.Msg) {
		log.Printf("[OCS] Session started: %s", string(m.Data))
		// TODO: Deduct quota if needed
	})
	if err != nil {
		log.Printf("Failed to subscribe to pfcp.session.created: %v", err)
	}

	// Initialize Redis
	InitRedis()

	// Create server with timeouts
	srv := &http.Server{
		Addr:         ":8084",
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Info("Starting OCS server on :8084")
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

func quotaHandler(w http.ResponseWriter, r *http.Request) {
	var req QuotaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.MB <= 0 {
		http.Error(w, "invalid quota amount", http.StatusBadRequest)
		return
	}

	if len(req.IMSI) < 10 {
		http.Error(w, "invalid IMSI format", http.StatusBadRequest)
		return
	}

	balanceLock.Lock()
	defer balanceLock.Unlock()

	sub, exists := balanceStore[req.IMSI]
	if !exists {
		json.NewEncoder(w).Encode(QuotaResponse{
			Approved: false,
			Reason:   "Subscriber not found",
		})
		return
	}

	if sub.Balance < req.MB {
		json.NewEncoder(w).Encode(QuotaResponse{
			Approved: false,
			Reason:   "Insufficient balance",
			Balance:  sub.Balance,
		})
		return
	}

	// Update balance
	sub.Balance -= req.MB
	sub.Updated = time.Now()
	sub.LastUsed = time.Now()

	log.WithFields(logrus.Fields{
		"imsi":      req.IMSI,
		"mb":        req.MB,
		"remaining": sub.Balance,
	}).Info("Quota approved and deducted")

	publisher.PublishQuotaDeducted(req.IMSI, req.MB, sub.Balance)

	// After quota deduction
	err = RedisClient.HSet(RedisCtx, "quota:"+req.IMSI, "remaining", sub.Balance).Err()
	if err != nil {
		log.Printf("Failed to store quota in Redis: %v", err)
	}

	json.NewEncoder(w).Encode(QuotaResponse{
		Approved: true,
		Balance:  sub.Balance,
	})
}

func getBalanceHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	imsi := vars["imsi"]

	balanceLock.RLock()
	defer balanceLock.RUnlock()

	sub, exists := balanceStore[imsi]
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
