package main

import (
	"encoding/asn1"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/ishidawataru/sctp"
	"github.com/nats-io/nats.go"
)

// NGAP message types
const (
	NGSetupRequestType   = 1
	NGSetupResponseType  = 2
	InitialUEMessageType = 3
)

// NGSetupRequest represents a simplified NGAP setup request
type NGSetupRequest struct {
	MessageType int
	UEID        uint64
	NASData     []byte
	Timestamp   time.Time
}

// NGSetupResponse represents a simplified NGAP setup response
type NGSetupResponse struct {
	MessageType int
	Success     bool
	Reason      string
	Timestamp   time.Time
}

// NAS represents a Non-Access Stratum message
type NAS struct {
	IMSI string
	Auth string
}

// NGAPMessage represents a simplified NGAP message
type NGAPMessage struct {
	MessageType int
	UEID        uint64
	NASMsg      NAS
	Timestamp   time.Time
}

// NGAPResponse represents the AMF's response
type NGAPResponse struct {
	MessageType int
	Success     bool
	Reason      string
	Timestamp   time.Time
}

// UEContext represents a UE's registration state
type UEContext struct {
	UEID      uint64
	GnbAddr   string
	IMSI      string
	AuthPass  bool
	Status    string
	LastSeen  time.Time
	CreatedAt time.Time
	Supi      string `json:"supi,omitempty"`     // Subscription Permanent Identifier
	AmfID     string `json:"amf_id,omitempty"`   // AMF Instance ID
	Guami     string `json:"guami,omitempty"`    // Globally Unique AMF ID
	PlmnID    string `json:"plmn_id,omitempty"`  // Public Land Mobile Network ID
	RatType   string `json:"rat_type,omitempty"` // Radio Access Technology Type
	CellID    string `json:"cell_id,omitempty"`  // Serving Cell ID
}

// UEStore manages UE contexts
type UEStore struct {
	store map[uint64]*UEContext
	mu    sync.RWMutex
}

// NewUEStore creates a new UE context store
func NewUEStore() *UEStore {
	return &UEStore{
		store: make(map[uint64]*UEContext),
	}
}

// Register adds or updates a UE context
func (s *UEStore) Register(ue *UEContext) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ue.LastSeen = time.Now()
	if ue.CreatedAt.IsZero() {
		ue.CreatedAt = ue.LastSeen
	}
	s.store[ue.UEID] = ue
}

// Get retrieves a UE context
func (s *UEStore) Get(ueid uint64) (*UEContext, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ue, ok := s.store[ueid]
	return ue, ok
}

// Delete removes a UE context
func (s *UEStore) Delete(ueid uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.store, ueid)
}

// List returns all UE contexts
func (s *UEStore) List() []*UEContext {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ues := make([]*UEContext, 0, len(s.store))
	for _, ue := range s.store {
		ues = append(ues, ue)
	}
	return ues
}

var (
	ueStore = NewUEStore()

	// In-memory UDM database
	udmDB = map[string]string{
		"001010123456789": "secret123",
		"001010987654321": "pass456",
		"001010111222333": "test789",
	}
)

func main() {
	addr := &sctp.SCTPAddr{
		IPAddrs: []net.IPAddr{{IP: net.ParseIP("0.0.0.0")}},
		Port:    38412,
	}

	l, err := sctp.ListenSCTP("sctp", addr)
	if err != nil {
		log.Fatalf("[AMF] Failed to bind SCTP: %v", err)
	}
	log.Println("[AMF] Listening on SCTP port 38412")

	// Connect to NATS
	nc, err := nats.Connect("nats://nats:4222")
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	publisher := NewPublisher(nc)

	for {
		conn, err := l.AcceptSCTP()
		if err != nil {
			log.Printf("[AMF] SCTP accept error: %v", err)
			continue
		}
		go handleNGAP(conn, publisher)
	}
}

func handleNGAP(conn *sctp.SCTPConn, publisher *Publisher) {
	defer conn.Close()
	peer := conn.RemoteAddr().String()
	log.Printf("[AMF] New connection from %s", peer)

	buf := make([]byte, 1024)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			log.Printf("[AMF] Read error from %s: %v", peer, err)
			return
		}

		var msg NGAPMessage
		if _, err := asn1.Unmarshal(buf[:n], &msg); err != nil {
			log.Printf("[AMF] ASN.1 decode failed from %s: %v", peer, err)
			continue
		}

		// Authenticate UE
		expectedAuth, ok := udmDB[msg.NASMsg.IMSI]
		authPassed := ok && msg.NASMsg.Auth == expectedAuth

		// Update UE context
		status := "REGISTERED"
		if !authPassed {
			status = "AUTH_FAILED"
		}

		ue := &UEContext{
			UEID:      msg.UEID,
			GnbAddr:   peer,
			IMSI:      msg.NASMsg.IMSI,
			AuthPass:  authPassed,
			Status:    status,
			LastSeen:  time.Now(),
			CreatedAt: time.Now(),
		}
		ueStore.Register(ue)

		// Send response
		resp := NGAPResponse{
			MessageType: 2, // Response type
			Success:     authPassed,
			Reason:      getAuthReason(authPassed, msg.NASMsg.IMSI),
			Timestamp:   time.Now(),
		}

		payload, err := asn1.Marshal(resp)
		if err != nil {
			log.Printf("[AMF] Failed to marshal response: %v", err)
			continue
		}

		_, err = conn.Write(payload)
		if err != nil {
			log.Printf("[AMF] Failed to send response: %v", err)
			continue
		}

		if authPassed {
			log.Printf("[AMF] UE %d (IMSI %s) authenticated ✅", msg.UEID, msg.NASMsg.IMSI)
			publisher.PublishUERegistered(fmt.Sprint(msg.UEID), msg.NASMsg.IMSI)
		} else {
			log.Printf("[AMF] UE %d (IMSI %s) failed auth ❌", msg.UEID, msg.NASMsg.IMSI)
		}
	}
}

func getAuthReason(authPassed bool, imsi string) string {
	if authPassed {
		return fmt.Sprintf("Authentication successful for IMSI %s", imsi)
	}
	if _, ok := udmDB[imsi]; !ok {
		return fmt.Sprintf("Unknown IMSI %s", imsi)
	}
	return fmt.Sprintf("Invalid authentication for IMSI %s", imsi)
}

// Start HTTP server for management API
func startHTTP() {
	r := mux.NewRouter()

	// API routes
	r.HandleFunc("/ue/{gnb_addr}", GetUE).Methods("GET")
	r.HandleFunc("/ue/{gnb_addr}", DeleteUE).Methods("DELETE")
	r.HandleFunc("/ue", ListUEs).Methods("GET")

	// Health check
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).Methods("GET")

	// Start HTTP server
	addr := ":8081"
	log.Printf("[AMF] Starting HTTP server on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("[AMF] HTTP server error: %v", err)
	}
}

// GetUE retrieves a UE's context
func GetUE(w http.ResponseWriter, r *http.Request) {
	gnbStr := mux.Vars(r)["gnb_addr"]
	gnbID, err := strconv.ParseUint(gnbStr, 10, 64)
	if err != nil {
		log.Printf("[AMF] Invalid gnb_addr format: %v", err)
		http.Error(w, "invalid gnb_addr", http.StatusBadRequest)
		return
	}

	ue, ok := ueStore.Get(gnbID)
	if !ok {
		log.Printf("[AMF] UE not found for gnbID %d", gnbID)
		http.Error(w, "UE not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(ue); err != nil {
		log.Printf("[AMF] Failed to encode UE context: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// DeleteUE removes a UE's context
func DeleteUE(w http.ResponseWriter, r *http.Request) {
	gnbStr := mux.Vars(r)["gnb_addr"]
	gnbID, err := strconv.ParseUint(gnbStr, 10, 64)
	if err != nil {
		log.Printf("[AMF] Invalid gnb_addr format: %v", err)
		http.Error(w, "invalid gnb_addr", http.StatusBadRequest)
		return
	}

	_, ok := ueStore.Get(gnbID)
	if !ok {
		log.Printf("[AMF] UE not found for gnbID %d", gnbID)
		http.Error(w, "UE not found", http.StatusNotFound)
		return
	}

	ueStore.Delete(gnbID)
	log.Printf("[AMF] Deleted UE %d", gnbID)
	w.WriteHeader(http.StatusNoContent)
}

// ListUEs returns all registered UEs
func ListUEs(w http.ResponseWriter, r *http.Request) {
	ues := ueStore.List()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ues)
}
