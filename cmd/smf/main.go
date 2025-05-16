package main

import (
	"context"
	"database/sql"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog"
	"github.com/openmvcore/pkg/smf"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"github.com/wmnsk/go-gtp/gtpv2"
	"github.com/wmnsk/go-gtp/gtpv2/ie"
	"github.com/wmnsk/go-gtp/gtpv2/message"
	_ "github.com/lib/pq"
)

var (
	configFile string
	config     *viper.Viper
	logger     *httplog.Logger

	// Network configuration
	GTPBindAddress = "0.0.0.0"
	GTPPort        = 2123
	IPPoolStart    = "10.0.0.1"
	IPPoolEnd      = "10.0.0.254"

	// Session configuration
	SessionTimeout = 24 * time.Hour
	DefaultQCI     = uint8(9)  // Default QoS Class Identifier
	DefaultARP     = uint8(1)  // Default Allocation and Retention Priority
)

// ----- Session Management -----
type Session struct {
	IMSI        string
	UEIP        net.IP
	TEID        uint32
	PeerAddr    string
	CreatedAt   time.Time
	LastUpdated time.Time
	State       SessionState
	BearerID    uint8
	QCI         uint8
	ARP         uint8
}

type SessionState string

const (
	SessionStateInitializing SessionState = "INITIALIZING"
	SessionStateActive       SessionState = "ACTIVE"
	SessionStateDeleting     SessionState = "DELETING"
	SessionStateDeleted      SessionState = "DELETED"
)

type SessionManager struct {
	sessions    map[string]*Session
	ipPool      []net.IP
	nextIPIndex int
	mu          sync.RWMutex
}

func NewSessionManager() *SessionManager {
	sm := &SessionManager{
		sessions: make(map[string]*Session),
	}
	sm.initIPPool()
	return sm
}

func (sm *SessionManager) initIPPool() {
	start := net.ParseIP(IPoolStart).To4()
	end := net.ParseIP(IPoolEnd).To4()
	
	for ip := start; !ip.Equal(end); ip = nextIP(ip) {
		sm.ipPool = append(sm.ipPool, append(net.IP(nil), ip...))
	}
}

func nextIP(ip net.IP) net.IP {
	next := make(net.IP, len(ip))
	copy(next, ip)
	for i := len(next) - 1; i >= 0; i-- {
		next[i]++
		if next[i] != 0 {
			break
		}
	}
	return next
}

func (sm *SessionManager) allocateIP() net.IP {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	if sm.nextIPIndex >= len(sm.ipPool) {
		return nil
	}
	ip := sm.ipPool[sm.nextIPIndex]
	sm.nextIPIndex++
	return ip
}

func (sm *SessionManager) createSession(imsi string, teid uint32, peerAddr string) (*Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check if session already exists
	if existing, ok := sm.sessions[imsi]; ok {
		return existing, nil
	}

	// Allocate new IP
	ueIP := sm.allocateIP()
	if ueIP == nil {
		return nil, fmt.Errorf("no IPs available in pool")
	}

	// Create new session
	session := &Session{
		IMSI:        imsi,
		UEIP:        ueIP,
		TEID:        teid,
		PeerAddr:    peerAddr,
		CreatedAt:   time.Now(),
		LastUpdated: time.Now(),
		State:       SessionStateInitializing,
		BearerID:    5, // Default EPS Bearer ID
		QCI:         DefaultQCI,
		ARP:         DefaultARP,
	}

	sm.sessions[imsi] = session
	return session, nil
}

func (sm *SessionManager) getSession(imsi string) (*Session, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	session, ok := sm.sessions[imsi]
	return session, ok
}

func (sm *SessionManager) deleteSession(imsi string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.sessions, imsi)
}

func init() {
	flag.StringVar(&configFile, "config", "configs/smf/config.yaml", "path to config file")
	flag.Parse()

	// Initialize configuration
	config = viper.New()
	config.SetConfigFile(configFile)
	config.AutomaticEnv()

	if err := config.ReadInConfig(); err != nil {
		panic(fmt.Sprintf("Error reading config file: %v", err))
	}

	// Initialize logger
	logger = httplog.NewLogger("smf", httplog.Options{
		JSON:    config.GetString("logging.format") == "json",
		Concise: true,
		Tags: map[string]string{
			"version": config.GetString("service.version"),
			"env":     config.GetString("service.environment"),
		},
	})
}

func main() {
	// Create context that listens for the interrupt signal
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Initialize databases
	redisClient := initRedis()
	pgDB := initPostgres()
	defer pgDB.Close()

	// Create session manager
	sessionManager = NewSessionManager()

	// Create GTP-C server
	gtpcAddr := fmt.Sprintf("%s:%d",
		config.GetString("interfaces.gtpc.ip"),
		config.GetInt("interfaces.gtpc.port"),
	)
	gtpcConn, err := net.ListenPacket("udp", gtpcAddr)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create GTP-C listener")
	}
	defer gtpcConn.Close()

	// Create GTP-C server instance
	gtpcServer := gtpv2.NewServer(gtpcConn)
	gtpcServer.AddHandlers(map[uint8]gtpv2.HandlerFunc{
		message.MsgTypeCreateSessionRequest: handleCreateSessionRequest,
		message.MsgTypeDeleteSessionRequest: handleDeleteSessionRequest,
	})

	// Start GTP-C server
	go func() {
		logger.Info().Str("addr", gtpcAddr).Msg("Starting GTP-C server")
		if err := gtpcServer.Serve(); err != nil {
			logger.Error().Err(err).Msg("GTP-C server error")
		}
	}()

	// Start metrics server (TODO)
	// Start PFCP server (TODO)

	// Wait for interrupt signal
	<-ctx.Done()
	logger.Info().Msg("Shutting down SMF service...")
}

// handleCreateSessionRequest processes incoming Create Session Requests
func handleCreateSessionRequest(c *gtpv2.Conn, senderAddr net.Addr, msg message.Message) error {
	req := msg.(*message.CreateSessionRequest)
	log.Printf("[GTP] Received CreateSessionRequest from %s", senderAddr.String())

	// Extract IMSI from the request
	imsiIE, err := req.IMSI()
	if err != nil {
		return fmt.Errorf("failed to get IMSI: %w", err)
	}
	imsi := imsiIE.String()

	// Create new session
	session, err := sessionManager.createSession(imsi, req.SenderTEID(), senderAddr.String())
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	// Create response message
	res := message.NewCreateSessionResponse(
		req.SenderTEID(), req.Sequence(),
		ie.NewCause(gtpv2.CauseRequestAccepted, 0, 0, 0, nil),
		ie.NewIMSI(imsi),
		ie.NewFullyQualifiedTEID(gtpv2.IFTypeS5S8SGWGTPC, session.TEID, c.LocalAddr().(*net.UDPAddr).IP, nil),
		ie.NewPDNAddressAllocation(session.UEIP),
		ie.NewAPNRestriction(gtpv2.APNRestrictionNoExistingContextsorRestrictions),
		ie.NewBearerContext(
			ie.NewEPSBearerID(session.BearerID),
			ie.NewBearerQoS(uint8(session.QCI), uint8(session.ARP), 0, 0, 0, 0),
		),
	)

	// Send response
	if err := c.RespondTo(senderAddr, req, res); err != nil {
		return fmt.Errorf("failed to send CreateSessionResponse: %w", err)
	}

	// Update session state
	session.State = SessionStateActive
	session.LastUpdated = time.Now()

	log.Printf("[SMF] Created session for IMSI %s with IP %s", imsi, session.UEIP)
	return nil
}

// handleDeleteSessionRequest processes incoming Delete Session Requests
func handleDeleteSessionRequest(c *gtpv2.Conn, senderAddr net.Addr, msg message.Message) error {
	req := msg.(*message.DeleteSessionRequest)
	log.Printf("[GTP] Received DeleteSessionRequest from %s", senderAddr.String())

	// Extract IMSI from the request
	imsiIE, err := req.IMSI()
	if err != nil {
		return fmt.Errorf("failed to get IMSI: %w", err)
	}
	imsi := imsiIE.String()

	// Get session
	session, ok := sessionManager.getSession(imsi)
	if !ok {
		return fmt.Errorf("session not found for IMSI: %s", imsi)
	}

	// Create response message
	res := message.NewDeleteSessionResponse(
		req.SenderTEID(), req.Sequence(),
		ie.NewCause(gtpv2.CauseRequestAccepted, 0, 0, 0, nil),
		ie.NewIMSI(imsi),
	)

	// Send response
	if err := c.RespondTo(senderAddr, req, res); err != nil {
		return fmt.Errorf("failed to send DeleteSessionResponse: %w", err)
	}

	// Delete session
	sessionManager.deleteSession(imsi)
	log.Printf("[SMF] Deleted session for IMSI %s", imsi)
	return nil
}

// initRedis initializes the Redis client
func initRedis() *redis.Client {
	redisClient := redis.NewClient(&redis.Options{
		Addr:         config.GetString("database.redis.addr"),
		DB:           config.GetInt("database.redis.db"),
		PoolSize:     config.GetInt("database.redis.pool_size"),
		MinIdleConns: 5,
		MaxRetries:   3,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to Redis")
	}

	logger.Info().Msg("Connected to Redis")
	return redisClient
}

// initPostgres initializes the PostgreSQL connection
func initPostgres() *sql.DB {
	db, err := sql.Open("postgres", config.GetString("database.postgres.dsn"))
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to PostgreSQL")
	}

	// Configure connection pool
	db.SetMaxOpenConns(config.GetInt("database.postgres.max_open_conns"))
	db.SetMaxIdleConns(config.GetInt("database.postgres.max_idle_conns"))
	db.SetConnMaxLifetime(config.GetDuration("database.postgres.conn_max_lifetime"))

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		logger.Fatal().Err(err).Msg("Failed to ping PostgreSQL")
	}

	logger.Info().Msg("Connected to PostgreSQL")
	return db
} 