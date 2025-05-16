package main

import (
	"encoding/json"
	"log"
	"time"

	"github.com/nats-io/nats.go"
)

// Publisher handles NATS message publishing
type Publisher struct {
	nc *nats.Conn
}

// NewPublisher creates a new NATS publisher
func NewPublisher(nc *nats.Conn) *Publisher {
	return &Publisher{nc: nc}
}

// PublishQuotaDeducted publishes a quota deduction event
func (p *Publisher) PublishQuotaDeducted(imsi string, deducted, remaining int) {
	event := map[string]interface{}{
		"imsi":      imsi,
		"deducted":  deducted,
		"remaining": remaining,
	}

	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("❌ Failed to marshal quota deduction event: %v", err)
		return
	}

	if err := p.nc.Publish("quota.deducted", data); err != nil {
		log.Printf("❌ Failed to publish quota deduction event: %v", err)
		return
	}

	log.Printf("✅ Published quota deduction event for IMSI %s (deducted: %d, remaining: %d)", imsi, deducted, remaining)
}

func (p *Publisher) PublishUERegistered(ueid, imsi string) {
	event := map[string]interface{}{
		"event":     "ue.registered",
		"ueid":      ueid,
		"imsi":      imsi,
		"timestamp": time.Now(),
	}
	p.publish("ue.registered", event)
}

func (p *Publisher) PublishPFCPCreated(sessionID, teid, ueIP string) {
	event := map[string]interface{}{
		"event":      "pfcp.session.created",
		"session_id": sessionID,
		"teid":       teid,
		"ue_ip":      ueIP,
		"timestamp":  time.Now(),
	}
	p.publish("pfcp.session.created", event)
}

func (p *Publisher) publish(subject string, msg any) {
	bytes, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[Publisher] Marshal error: %v", err)
		return
	}
	_, err = p.nc.Publish(subject, bytes)
	if err != nil {
		log.Printf("[Publisher] Failed to publish %s: %v", subject, err)
	}
}
