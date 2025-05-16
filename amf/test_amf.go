package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type TestUE struct {
	IMSI     string    `json:"imsi"`
	IP       string    `json:"ip"`
	Created  time.Time `json:"created"`
	LastSeen time.Time `json:"last_seen"`
}

func TestRedisSessionCache(t *testing.T) {
	// Initialize Redis
	InitRedis()
	defer RedisClient.Close()

	// Test data
	ue := TestUE{
		IMSI:     "001010123456789",
		IP:       "192.168.1.100",
		Created:  time.Now(),
		LastSeen: time.Now(),
	}

	// Marshal UE to JSON
	ueJSON, err := json.Marshal(ue)
	assert.NoError(t, err)

	// Store in Redis
	err = RedisClient.Set(RedisCtx, "ue:"+ue.IMSI, ueJSON, 1*time.Minute).Err()
	assert.NoError(t, err)

	// Retrieve from Redis
	val, err := RedisClient.Get(RedisCtx, "ue:"+ue.IMSI).Result()
	assert.NoError(t, err)

	// Unmarshal and verify
	var retrievedUE TestUE
	err = json.Unmarshal([]byte(val), &retrievedUE)
	assert.NoError(t, err)
	assert.Equal(t, ue.IMSI, retrievedUE.IMSI)
	assert.Equal(t, ue.IP, retrievedUE.IP)
}

func TestRedisConnection(t *testing.T) {
	// Test Redis connection
	InitRedis()
	defer RedisClient.Close()

	// Simple ping test
	err := RedisClient.Ping(RedisCtx).Err()
	assert.NoError(t, err)
}
