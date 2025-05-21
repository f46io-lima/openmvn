package main

import (
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/wmnsk/go-pfcp/message"
)

type TestPFCPTunnel struct {
	TEID     uint32    `json:"teid"`
	UEIP     net.IP    `json:"ue_ip"`
	Created  time.Time `json:"created"`
	State    string    `json:"state"`
	SEID     uint64    `json:"seid"`
	QFI      uint8     `json:"qfi"`
	Priority uint8     `json:"priority"`
}

func TestRedisPFCPStorage(t *testing.T) {
	// Initialize Redis
	InitRedis()
	defer RedisClient.Close()

	// Test data
	tunnel := TestPFCPTunnel{
		TEID:     0x12345678,
		UEIP:     net.ParseIP("192.168.1.100"),
		Created:  time.Now(),
		State:    "active",
		SEID:     0x1234567890abcdef,
		QFI:      1,
		Priority: 255,
	}

	// Marshal tunnel to JSON
	tunnelJSON, err := json.Marshal(tunnel)
	assert.NoError(t, err)

	// Store in Redis with proper string conversion
	redisKey := fmt.Sprintf("pfcp:%d", tunnel.TEID)
	err = RedisClient.Set(RedisCtx, redisKey, tunnelJSON, 30*time.Minute).Err()
	assert.NoError(t, err)

	// Retrieve from Redis
	val, err := RedisClient.Get(RedisCtx, redisKey).Result()
	assert.NoError(t, err)

	// Unmarshal and verify
	var retrievedTunnel TestPFCPTunnel
	err = json.Unmarshal([]byte(val), &retrievedTunnel)
	assert.NoError(t, err)
	assert.Equal(t, tunnel.TEID, retrievedTunnel.TEID)
	assert.Equal(t, tunnel.UEIP.String(), retrievedTunnel.UEIP.String())
	assert.Equal(t, tunnel.State, retrievedTunnel.State)
	assert.Equal(t, tunnel.SEID, retrievedTunnel.SEID)
	assert.Equal(t, tunnel.QFI, retrievedTunnel.QFI)
	assert.Equal(t, tunnel.Priority, retrievedTunnel.Priority)
}

func TestPFCPMessageParsing(t *testing.T) {
	// Skip in CI environment
	if testing.Short() {
		t.Skip("Skipping PFCP message parsing test in short mode")
	}

	// Test PFCP message parsing
	// This is a basic test to ensure we can parse PFCP messages
	// In a real test, we would create actual PFCP messages
	msg := message.NewSessionEstablishmentRequest(0, 0, 1, 1, 0, nil, nil, nil, nil)
	_, err := msg.Marshal()
	assert.NoError(t, err)
}
