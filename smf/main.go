package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog"
	"github.com/go-redis/redis/v8"
	"github.com/nats-io/nats.go"
	"github.com/spf13/viper"
	"github.com/wmnsk/go-gtp/gtpv2"
	gtpie "github.com/wmnsk/go-gtp/gtpv2/ie"
	"github.com/wmnsk/go-gtp/gtpv2/message"
	pfcp_ie "github.com/wmnsk/go-pfcp/ie"
	pfcpmsg "github.com/wmnsk/go-pfcp/message"
)

var (
	logger = httplog.NewLogger("smf", httplog.Options{
		JSON:    true,
		Concise: true,
	})
	redisClient *redis.Client
	upfConn     *net.UDPConn
)

// PFCPClient represents a PFCP client for UPF communication
type PFCPClient struct {
	conn     *net.UDPConn
	upfAddr  string
	sessions map[uint64]*gtpv2.Session
}

// NewPFCPClient creates a new PFCP client
func NewPFCPClient(upfAddr string) (*PFCPClient, error) {
	conn, err := net.Dial("udp", upfAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to UPF PFCP at %s: %v", upfAddr, err)
	}

	return &PFCPClient{
		conn:     conn.(*net.UDPConn),
		upfAddr:  upfAddr,
		sessions: make(map[uint64]*gtpv2.Session),
	}, nil
}

// CreateSession sends a CreateSessionRequest to the UPF
func (c *PFCPClient) CreateSession(seid uint64, ueIP net.IP, teid uint32) error {
	// Create PFCP IEs
	nodeID := pfcp_ie.NewNodeID("smf.local", "", "")
	fseid := pfcp_ie.NewFSEID(seid, net.ParseIP("127.0.0.1"), nil)
	pdrID := pfcp_ie.NewPDRID(1)
	precedence := pfcp_ie.NewPrecedence(255)
	sourceInterface := pfcp_ie.NewSourceInterface(pfcp_ie.SrcInterfaceAccess)
	ueIPAddr := pfcp_ie.NewUEIPAddress(0, ueIP.String(), "", 0, 0)
	fteid := pfcp_ie.NewFTEID(0, teid, net.ParseIP("127.0.0.1"), nil, 0)
	outerHeaderRemoval := pfcp_ie.NewOuterHeaderRemoval(0, 0)
	farID := pfcp_ie.NewFARID(1)
	applyAction := pfcp_ie.NewApplyAction(2) // Forward
	destinationInterface := pfcp_ie.NewDestinationInterface(pfcp_ie.DstInterfaceCore)

	// Create PDI
	pdi := pfcp_ie.NewPDI(sourceInterface, ueIPAddr, fteid)

	// Create CreatePDR
	createPDR := pfcp_ie.NewCreatePDR(pdrID, precedence, pdi, outerHeaderRemoval)

	// Create ForwardingParameters
	forwardingParams := pfcp_ie.NewForwardingParameters(destinationInterface)

	// Create CreateFAR
	createFAR := pfcp_ie.NewCreateFAR(farID, applyAction, forwardingParams)

	// Create CreateSessionRequest
	pfcpMsg := pfcpmsg.NewSessionEstablishmentRequest(0, 0, seid, 1, 0, nodeID, fseid, createPDR, createFAR)

	buf, err := pfcpMsg.Marshal()
	if err != nil {
		return fmt.Errorf("marshal error: %v", err)
	}

	n, err := c.conn.Write(buf)
	if err != nil {
		return fmt.Errorf("write error: %v", err)
	}

	logger.Info().Msgf("[SMF] Sent PFCP CreateSessionRequest (%d bytes)", n)

	// Wait for UPF reply
	c.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	resp := make([]byte, 1500)
	n, err = c.conn.Read(resp)
	if err != nil {
		return fmt.Errorf("no PFCP response: %v", err)
	}

	parsed, err := pfcpmsg.Parse(resp[:n])
	if err != nil {
		return fmt.Errorf("parse response failed: %v", err)
	}

	if _, ok := parsed.(*pfcpmsg.SessionEstablishmentResponse); ok {
		logger.Info().Msg("[SMF] Received SessionEstablishmentResponse ✅")
		return nil
	}

	return fmt.Errorf("unexpected PFCP message: %T", parsed)
}

func main() {
	// Set default configuration
	viper.SetDefault("redis.addr", "localhost:6379")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("upf.addr", "127.0.0.1:8805")

	// Load configuration
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			logger.Warn().Msg("No config file found, using defaults")
		} else {
			logger.Fatal().Err(err).Msg("Failed to read config file")
		}
	}

	// Initialize Redis client
	redisClient = redis.NewClient(&redis.Options{
		Addr:     viper.GetString("redis.addr"),
		Password: viper.GetString("redis.password"),
		DB:       viper.GetInt("redis.db"),
	})

	// Test Redis connection
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		logger.Warn().Err(err).Msg("Failed to connect to Redis, continuing without Redis")
		redisClient = nil
	}

	// Initialize PFCP client
	pfcpClient, err := NewPFCPClient(viper.GetString("upf.addr"))
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize PFCP client")
	}
	defer pfcpClient.conn.Close()

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

	// Subscribe to UE registration events
	_, err = js.Subscribe("ue.registered", func(m *nats.Msg) {
		log.Printf("[SMF] New UE Registered: %s", string(m.Data))
		// TODO: Parse and prebuild session if needed
	})
	if err != nil {
		log.Printf("Failed to subscribe to ue.registered: %v", err)
	}

	// Initialize router
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(httplog.RequestLogger(logger))
	r.Use(middleware.Recoverer)

	// Handle GTP-C messages
	r.Post("/gtpc/v1/create-session", func(w http.ResponseWriter, r *http.Request) {
		// Read request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to read request body")
			http.Error(w, "Failed to read request", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Parse GTP-C message
		msg, err := message.Parse(body)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to parse GTP-C message")
			http.Error(w, "Invalid message format", http.StatusBadRequest)
			return
		}

		// Handle Create Session Request
		if csReq, ok := msg.(*message.CreateSessionRequest); ok {
			// Extract session parameters
			imsi := csReq.IMSI.String()

			ueIP, err := csReq.PAA.IP()
			if err != nil {
				logger.Error().Err(err).Msg("Failed to get UE IP")
				http.Error(w, "Invalid UE IP", http.StatusBadRequest)
				return
			}

			teid, err := csReq.SenderFTEIDC.TEID()
			if err != nil {
				logger.Error().Err(err).Msg("Failed to get Sender F-TEID")
				http.Error(w, "Invalid F-TEID", http.StatusBadRequest)
				return
			}

			logger.Info().
				Str("imsi", imsi).
				Str("ue_ip", ueIP.String()).
				Uint32("teid", teid).
				Msg("Received Create Session Request")

			// Create PFCP session
			if err := pfcpClient.CreateSession(1, ueIP, teid); err != nil {
				logger.Error().Err(err).Msg("Failed to create PFCP session")
				http.Error(w, "PFCP session creation failed", http.StatusInternalServerError)
				return
			}

			// Create success response
			csResp := message.NewCreateSessionResponse(
				csReq.Sequence(),
				csReq.TEID(),
				gtpie.NewCause(gtpv2.CauseRequestAccepted, 0, 0, 0, nil),
			)

			// Send response
			respBuf, err := csResp.Marshal()
			if err != nil {
				logger.Error().Err(err).Msg("Failed to marshal response")
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write(respBuf); err != nil {
				logger.Error().Err(err).Msg("Failed to write response")
			}

			// Publish PFCP session created event
			publisher.PublishPFCPCreated(
				fmt.Sprintf("%d", teid), // Convert teid (uint32) to string
				ueIP.String(),           // Convert net.IP to string
				"",                      // (or pass a valid session ID if available)
			)

			// TODO: Store session details in Redis if needed
			// For now, we'll skip storing session details until we have a session struct defined

			return
		}

		logger.Warn().Type("message_type", msg).Msg("Unsupported message type")
		http.Error(w, "Unsupported message type", http.StatusBadRequest)
	})

	// Add health check endpoint
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Start HTTP server
	server := &http.Server{
		Addr:    ":2123",
		Handler: r,
	}

	// Handle graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info().Msg("[SMF] Server starting on :2123")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("HTTP server error")
		}
	}()

	<-ctx.Done()
	logger.Info().Msg("[SMF] Shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("Server shutdown error")
	}
}
