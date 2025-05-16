package main

import (
	"encoding/json"
	"log"
	"time"

	"github.com/nats-io/nats.go"
)

type EventPublisher struct {
	js nats.JetStreamContext
}

func NewPublisher(nc *nats.Conn) *EventPublisher {
	js, _ := nc.JetStream()
	return &EventPublisher{js: js}
}

func (p *EventPublisher) PublishUERegistered(ueid, imsi string) {
	event := map[string]interface{}{
		"event":     "ue.registered",
		"ueid":      ueid,
		"imsi":      imsi,
		"timestamp": time.Now(),
	}
	p.publish("ue.registered", event)
}

func (p *EventPublisher) PublishPFCPCreated(sessionID, teid, ueIP string) {
	event := map[string]interface{}{
		"event":      "pfcp.session.created",
		"session_id": sessionID,
		"teid":       teid,
		"ue_ip":      ueIP,
		"timestamp":  time.Now(),
	}
	p.publish("pfcp.session.created", event)
}

func (p *EventPublisher) PublishQuotaDeducted(imsi string, amount, remaining int) {
	event := map[string]interface{}{
		"event":     "quota.deducted",
		"imsi":      imsi,
		"amount_mb": amount,
		"remaining": remaining,
		"timestamp": time.Now(),
	}
	p.publish("quota.deducted", event)
}

func (p *EventPublisher) publish(subject string, msg any) {
	bytes, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[Publisher] Marshal error: %v", err)
		return
	}
	_, err = p.js.Publish(subject, bytes)
	if err != nil {
		log.Printf("[Publisher] Failed to publish %s: %v", subject, err)
	}
}
