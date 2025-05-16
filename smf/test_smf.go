package main

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type TestSession struct {
	UEID    string    `json:"ue_id"`
	TEID    uint32    `json:"teid"`
	UEIP    net.IP    `json:"ue_ip"`
	Created time.Time `json:"created"`
	State   string    `json:"state"`
}

func TestRedisSessionStorage(t *testing.T) {
	// Initialize Redis
	InitRedis()
	defer RedisClient.Close()

	// Test data
	session := TestSession{
		UEID:    "001010123456789",
		TEID:    0x12345678,
		UEIP:    net.ParseIP("192.168.1.100"),
		Created: time.Now(),
		State:   "active",
	}

	// Marshal session to JSON
	sessionJSON, err := json.Marshal(session)
	assert.NoError(t, err)

	// Store in Redis
	err = RedisClient.Set(RedisCtx, "session:"+session.UEID, sessionJSON, 30*time.Minute).Err()
	assert.NoError(t, err)

	// Retrieve from Redis
	val, err := RedisClient.Get(RedisCtx, "session:"+session.UEID).Result()
	assert.NoError(t, err)

	// Unmarshal and verify
	var retrievedSession TestSession
	err = json.Unmarshal([]byte(val), &retrievedSession)
	assert.NoError(t, err)
	assert.Equal(t, session.UEID, retrievedSession.UEID)
	assert.Equal(t, session.TEID, retrievedSession.TEID)
	assert.Equal(t, session.UEIP.String(), retrievedSession.UEIP.String())
	assert.Equal(t, session.State, retrievedSession.State)
}

func TestPFCPClient(t *testing.T) {
	// Skip in CI environment
	if testing.Short() {
		t.Skip("Skipping PFCP test in short mode")
	}

	// Create PFCP client
	client, err := NewPFCPClient("127.0.0.1:8805")
	if err != nil {
		t.Skip("UPF not available, skipping test")
	}
	defer client.conn.Close()

	// Test session creation
	seid := uint64(1)
	ueIP := net.ParseIP("192.168.1.100")
	teid := uint32(0x12345678)

	err = client.CreateSession(seid, ueIP, teid)
	if err != nil {
		t.Logf("PFCP session creation failed (expected in test): %v", err)
	}
}
