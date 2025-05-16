package smf

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/wmnsk/go-gtp/gtpv2"
	"github.com/wmnsk/go-gtp/gtpv2/message"
)

// Session represents a GTP-C session
type Session struct {
	// Session identifiers
	IMSI        string    `json:"imsi"`
	TEID        uint32    `json:"teid"`
	SessionID   string    `json:"session_id"`
	CreatedAt   time.Time `json:"created_at"`
	LastUpdated time.Time `json:"last_updated"`

	// Session state
	State SessionState `json:"state"`

	// Bearer information
	BearerID uint8  `json:"bearer_id"`
	QoS      *QoS   `json:"qos"`
	APN      string `json:"apn"`

	// UPF information
	UPFNodeID string `json:"upf_node_id"`
	UPFTEID   uint32 `json:"upf_teid"`

	// PFCP session
	PFCPFSEID uint64 `json:"pfcp_fseid"`

	// Connection information
	AMFAddr string `json:"amf_addr"`
	UPFAddr string `json:"upf_addr"`

	mu sync.RWMutex
}

// SessionState represents the state of a GTP-C session
type SessionState string

const (
	SessionStateInitializing SessionState = "INITIALIZING"
	SessionStateActive       SessionState = "ACTIVE"
	SessionStateModifying    SessionState = "MODIFYING"
	SessionStateDeleting     SessionState = "DELETING"
	SessionStateDeleted      SessionState = "DELETED"
)

// QoS represents the Quality of Service parameters
type QoS struct {
	QCI                uint8  `json:"qci"`
	ARP                uint8  `json:"arp"`
	PriorityLevel      uint8  `json:"priority_level"`
	PreemptionCap      bool   `json:"preemption_cap"`
	PreemptionVuln     bool   `json:"preemption_vuln"`
	MaxBitRateUL       uint64 `json:"max_bit_rate_ul"`
	MaxBitRateDL       uint64 `json:"max_bit_rate_dl"`
	GuaranteedBitRateUL uint64 `json:"guaranteed_bit_rate_ul"`
	GuaranteedBitRateDL uint64 `json:"guaranteed_bit_rate_dl"`
}

// SessionManager handles GTP-C session management
type SessionManager struct {
	redis *redis.Client
	mu    sync.RWMutex
}

// NewSessionManager creates a new session manager
func NewSessionManager(redis *redis.Client) *SessionManager {
	return &SessionManager{
		redis: redis,
	}
}

// CreateSession creates a new GTP-C session
func (sm *SessionManager) CreateSession(ctx context.Context, imsi string, teid uint32, amfAddr string) (*Session, error) {
	session := &Session{
		IMSI:        imsi,
		TEID:        teid,
		SessionID:   uuid.New().String(),
		CreatedAt:   time.Now(),
		LastUpdated: time.Now(),
		State:       SessionStateInitializing,
		AMFAddr:     amfAddr,
	}

	// Store session in Redis
	if err := sm.storeSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to store session: %w", err)
	}

	return session, nil
}

// GetSession retrieves a session by IMSI
func (sm *SessionManager) GetSession(ctx context.Context, imsi string) (*Session, error) {
	key := fmt.Sprintf("session:%s", imsi)
	data, err := sm.redis.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("session not found for IMSI: %s", imsi)
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &session, nil
}

// UpdateSession updates an existing session
func (sm *SessionManager) UpdateSession(ctx context.Context, session *Session) error {
	session.LastUpdated = time.Now()
	return sm.storeSession(ctx, session)
}

// DeleteSession deletes a session
func (sm *SessionManager) DeleteSession(ctx context.Context, imsi string) error {
	key := fmt.Sprintf("session:%s", imsi)
	return sm.redis.Del(ctx, key).Err()
}

// storeSession stores a session in Redis
func (sm *SessionManager) storeSession(ctx context.Context, session *Session) error {
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	key := fmt.Sprintf("session:%s", session.IMSI)
	return sm.redis.Set(ctx, key, data, 24*time.Hour).Err()
}

// HandleCreateSessionRequest processes a Create Session Request
func (sm *SessionManager) HandleCreateSessionRequest(ctx context.Context, msg *message.CreateSessionRequest) (*Session, error) {
	// Extract IMSI from the request
	imsi, err := msg.IMSI()
	if err != nil {
		return nil, fmt.Errorf("failed to get IMSI: %w", err)
	}

	// Check if session already exists
	if existing, err := sm.GetSession(ctx, imsi); err == nil {
		return existing, nil
	}

	// Create new session
	session, err := sm.CreateSession(ctx, imsi, msg.SenderTEID(), msg.SenderAddr().String())
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// TODO: Extract and set additional session parameters from the request
	// - Bearer ID
	// - QoS
	// - APN
	// - UPF selection
	// - PFCP session creation

	return session, nil
}

// HandleDeleteSessionRequest processes a Delete Session Request
func (sm *SessionManager) HandleDeleteSessionRequest(ctx context.Context, msg *message.DeleteSessionRequest) error {
	// Extract IMSI from the request
	imsi, err := msg.IMSI()
	if err != nil {
		return fmt.Errorf("failed to get IMSI: %w", err)
	}

	// Get existing session
	session, err := sm.GetSession(ctx, imsi)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// Update session state
	session.State = SessionStateDeleting
	session.LastUpdated = time.Now()

	// TODO: Clean up PFCP session
	// TODO: Release UPF resources

	// Delete session
	return sm.DeleteSession(ctx, imsi)
}

// HandleModifyBearerRequest processes a Modify Bearer Request
func (sm *SessionManager) HandleModifyBearerRequest(ctx context.Context, msg *message.ModifyBearerRequest) error {
	// Extract IMSI from the request
	imsi, err := msg.IMSI()
	if err != nil {
		return fmt.Errorf("failed to get IMSI: %w", err)
	}

	// Get existing session
	session, err := sm.GetSession(ctx, imsi)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// Update session state
	session.State = SessionStateModifying
	session.LastUpdated = time.Now()

	// TODO: Update bearer information
	// TODO: Update QoS if changed
	// TODO: Update UPF if needed

	// Store updated session
	return sm.UpdateSession(ctx, session)
} 