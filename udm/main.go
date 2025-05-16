package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
)

type AuthRequest struct {
	IMSI  string `json:"imsi"`
	Token string `json:"token"`
}

type AuthResponse struct {
	Authorized bool   `json:"authorized"`
	Reason     string `json:"reason,omitempty"`
}

// In-memory database (replace with PostgreSQL later)
var db = map[string]string{
	"001010123456789": "secret123",
	"001010987654321": "pass456",
}

func main() {
	// Initialize router
	r := mux.NewRouter()

	// Add middleware
	r.Use(loggingMiddleware)
	r.Use(recoveryMiddleware)

	// Register routes
	r.HandleFunc("/auth", authHandler).Methods("POST")
	r.HandleFunc("/health", healthHandler).Methods("GET")

	// Create server with timeouts
	srv := &http.Server{
		Addr:         ":8082",
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Println("[UDM] Starting server on :8082")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[UDM] Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Graceful shutdown
	log.Println("[UDM] Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("[UDM] Server forced to shutdown: %v", err)
	}

	log.Println("[UDM] Server exited properly")
}

func authHandler(w http.ResponseWriter, r *http.Request) {
	var req AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	// Validate IMSI format (basic check)
	if len(req.IMSI) < 10 {
		http.Error(w, "invalid IMSI format", http.StatusBadRequest)
		return
	}

	expected, exists := db[req.IMSI]
	authorized := exists && expected == req.Token

	resp := AuthResponse{
		Authorized: authorized,
	}
	if !authorized {
		resp.Reason = "Invalid IMSI or token"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("[UDM] %s %s %s", r.Method, r.RequestURI, time.Since(start))
	})
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("[UDM] Panic recovered: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
} 