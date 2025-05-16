package upf

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	gtpv1msg "github.com/wmnsk/go-gtp/gtpv1/message"
	"github.com/wmnsk/go-pfcp/ie"
	pfcpmsg "github.com/wmnsk/go-pfcp/message"
)

// UPF represents a User Plane Function instance
type UPF struct {
	cfg *Config

	// PFCP server
	pfcpConn *net.UDPConn
	pfcpAddr *net.UDPAddr

	// GTP-U server
	gtpuConn *net.UDPConn
	gtpuAddr *net.UDPAddr

	// Session management
	sessions    map[uint64]*Session
	sessionLock sync.RWMutex

	// Logging
	logger *logrus.Logger
}

// Config holds UPF configuration
type Config struct {
	PFCP struct {
		Addr string
	}
	GTP struct {
		Addr string
	}
	EnableGTP    bool
	EnablePFCP   bool
	EnableUPlane bool
	ReportNotify bool
	LogLevel     string
}

// Session represents a PFCP session
type Session struct {
	SEID      uint64
	UEIP      net.IP
	TEID      uint32
	CreatedAt time.Time
	UpdatedAt time.Time
	State     string
}

// NewUPF creates a new UPF instance
func NewUPF(cfg *Config) *UPF {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339,
	})
	logger.SetOutput(logrus.StandardLogger().Out)

	level, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	return &UPF{
		cfg:      cfg,
		sessions: make(map[uint64]*Session),
		logger:   logger,
	}
}

// Run starts the UPF services
func (u *UPF) Run() error {
	if u.cfg.EnablePFCP {
		if err := u.startPFCP(); err != nil {
			return fmt.Errorf("failed to start PFCP: %w", err)
		}
	}

	if u.cfg.EnableGTP {
		if err := u.startGTP(); err != nil {
			return fmt.Errorf("failed to start GTP: %w", err)
		}
	}

	return nil
}

// Close shuts down the UPF
func (u *UPF) Close() {
	if u.pfcpConn != nil {
		u.pfcpConn.Close()
	}
	if u.gtpuConn != nil {
		u.gtpuConn.Close()
	}
}

// GetSessionCount returns the number of active sessions
func (u *UPF) GetSessionCount() int {
	u.sessionLock.RLock()
	defer u.sessionLock.RUnlock()
	return len(u.sessions)
}

// startPFCP starts the PFCP server
func (u *UPF) startPFCP() error {
	var err error
	u.pfcpAddr, err = net.ResolveUDPAddr("udp", u.cfg.PFCP.Addr)
	if err != nil {
		return fmt.Errorf("invalid PFCP address: %w", err)
	}

	u.pfcpConn, err = net.ListenUDP("udp", u.pfcpAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on PFCP: %w", err)
	}

	go u.servePFCP()
	u.logger.Infof("[UPF] PFCP server listening on %s", u.pfcpAddr)
	return nil
}

// startGTP starts the GTP-U server
func (u *UPF) startGTP() error {
	var err error
	u.gtpuAddr, err = net.ResolveUDPAddr("udp", u.cfg.GTP.Addr)
	if err != nil {
		return fmt.Errorf("invalid GTP address: %w", err)
	}

	u.gtpuConn, err = net.ListenUDP("udp", u.gtpuAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on GTP: %w", err)
	}

	go u.serveGTP()
	u.logger.Infof("[UPF] GTP-U server listening on %s", u.gtpuAddr)
	return nil
}

// servePFCP handles incoming PFCP messages
func (u *UPF) servePFCP() {
	buf := make([]byte, 1500)
	for {
		n, remoteAddr, err := u.pfcpConn.ReadFromUDP(buf)
		if err != nil {
			u.logger.Errorf("[UPF] PFCP read error: %v", err)
			continue
		}

		msg, err := pfcpmsg.Parse(buf[:n])
		if err != nil {
			u.logger.Errorf("[UPF] Failed to parse PFCP message: %v", err)
			continue
		}

		go u.handlePFCP(msg, remoteAddr)
	}
}

// serveGTP handles incoming GTP-U messages
func (u *UPF) serveGTP() {
	buf := make([]byte, 1500)
	for {
		n, remoteAddr, err := u.gtpuConn.ReadFromUDP(buf)
		if err != nil {
			u.logger.Errorf("[UPF] GTP-U read error: %v", err)
			continue
		}

		msg, err := gtpv1msg.Parse(buf[:n])
		if err != nil {
			u.logger.Errorf("[UPF] Failed to parse GTP-U message: %v", err)
			continue
		}

		go u.handleGTP(msg, remoteAddr)
	}
}

// handlePFCP processes PFCP messages
func (u *UPF) handlePFCP(msg pfcpmsg.Message, remoteAddr *net.UDPAddr) {
	switch m := msg.(type) {
	case *pfcpmsg.HeartbeatRequest:
		u.handleHeartbeatRequest(m, remoteAddr)
	case *pfcpmsg.AssociationSetupRequest:
		u.handleAssociationSetupRequest(m, remoteAddr)
	case *pfcpmsg.SessionEstablishmentRequest:
		u.handleSessionEstablishmentRequest(m, remoteAddr)
	case *pfcpmsg.SessionModificationRequest:
		u.handleSessionModificationRequest(m, remoteAddr)
	case *pfcpmsg.SessionDeletionRequest:
		u.handleSessionDeletionRequest(m, remoteAddr)
	default:
		u.logger.Warnf("[UPF] Unhandled PFCP message type: %T", msg)
	}
}

// handleGTP processes GTP-U messages
func (u *UPF) handleGTP(msg gtpv1msg.Message, remoteAddr *net.UDPAddr) {
	// TODO: Implement GTP-U packet handling
	u.logger.Debugf("[UPF] Received GTP-U message from %s: %+v", remoteAddr, msg)
}

// handleHeartbeatRequest processes PFCP Heartbeat Request
func (u *UPF) handleHeartbeatRequest(req *pfcpmsg.HeartbeatRequest, remoteAddr *net.UDPAddr) {
	res := pfcpmsg.NewHeartbeatResponse(
		req.SequenceNumber,
		ie.NewRecoveryTimeStamp(time.Now()),
	)

	if err := u.sendPFCP(res, remoteAddr); err != nil {
		u.logger.Errorf("[UPF] Failed to send Heartbeat Response: %v", err)
	}
}

// handleAssociationSetupRequest processes PFCP Association Setup Request
func (u *UPF) handleAssociationSetupRequest(req *pfcpmsg.AssociationSetupRequest, remoteAddr *net.UDPAddr) {
	res := pfcpmsg.NewAssociationSetupResponse(
		req.SequenceNumber,
		ie.NewNodeID("upf.local", "", ""),
		ie.NewCause(ie.CauseRequestAccepted),
		ie.NewRecoveryTimeStamp(time.Now()),
	)

	if err := u.sendPFCP(res, remoteAddr); err != nil {
		u.logger.Errorf("[UPF] Failed to send Association Setup Response: %v", err)
	}
}

// handleSessionEstablishmentRequest processes PFCP Session Establishment Request
func (u *UPF) handleSessionEstablishmentRequest(req *pfcpmsg.SessionEstablishmentRequest, remoteAddr *net.UDPAddr) {
	// Extract session information
	seid := req.SEID()

	// Create new session
	session := &Session{
		SEID:      seid,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		State:     "ESTABLISHED",
	}

	u.sessionLock.Lock()
	u.sessions[seid] = session
	u.sessionLock.Unlock()

	// Send response
	res := pfcpmsg.NewSessionEstablishmentResponse(
		uint8(req.SequenceNumber&0xFF),
		uint8(req.MessagePriority),
		seid,
		req.SequenceNumber,
		uint8(req.MessagePriority),
		ie.NewNodeID("upf.local", "", ""),
		ie.NewCause(ie.CauseRequestAccepted),
		ie.NewRecoveryTimeStamp(time.Now()),
		ie.NewFSEID(seid, net.ParseIP("127.0.0.1"), nil),
	)

	if err := u.sendPFCP(res, remoteAddr); err != nil {
		u.logger.Errorf("[UPF] Failed to send Session Establishment Response: %v", err)
	}
}

// handleSessionModificationRequest processes PFCP Session Modification Request
func (u *UPF) handleSessionModificationRequest(req *pfcpmsg.SessionModificationRequest, remoteAddr *net.UDPAddr) {
	seid := req.SEID()

	u.sessionLock.Lock()
	session, exists := u.sessions[seid]
	if exists {
		session.UpdatedAt = time.Now()
	}
	u.sessionLock.Unlock()

	if !exists {
		u.logger.Errorf("[UPF] Session %d not found", seid)
		return
	}

	res := pfcpmsg.NewSessionModificationResponse(
		uint8(req.SequenceNumber&0xFF),
		uint8(req.MessagePriority),
		seid,
		req.SequenceNumber,
		uint8(req.MessagePriority),
		ie.NewNodeID("upf.local", "", ""),
		ie.NewCause(ie.CauseRequestAccepted),
	)

	if err := u.sendPFCP(res, remoteAddr); err != nil {
		u.logger.Errorf("[UPF] Failed to send Session Modification Response: %v", err)
	}
}

// handleSessionDeletionRequest processes PFCP Session Deletion Request
func (u *UPF) handleSessionDeletionRequest(req *pfcpmsg.SessionDeletionRequest, remoteAddr *net.UDPAddr) {
	seid := req.SEID()

	u.sessionLock.Lock()
	delete(u.sessions, seid)
	u.sessionLock.Unlock()

	res := pfcpmsg.NewSessionDeletionResponse(
		uint8(req.SequenceNumber&0xFF),
		uint8(req.MessagePriority),
		seid,
		req.SequenceNumber,
		uint8(req.MessagePriority),
		ie.NewNodeID("upf.local", "", ""),
		ie.NewCause(ie.CauseRequestAccepted),
	)

	if err := u.sendPFCP(res, remoteAddr); err != nil {
		u.logger.Errorf("[UPF] Failed to send Session Deletion Response: %v", err)
	}
}

// sendPFCP sends a PFCP message to the specified address
func (u *UPF) sendPFCP(msg pfcpmsg.Message, remoteAddr *net.UDPAddr) error {
	buf := make([]byte, msg.MarshalLen())
	msg.MarshalTo(buf)

	_, err := u.pfcpConn.WriteToUDP(buf, remoteAddr)
	if err != nil {
		return fmt.Errorf("failed to send PFCP message: %w", err)
	}

	return nil
}
