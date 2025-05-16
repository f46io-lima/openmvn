package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRedisQuotaStorage(t *testing.T) {
	// Initialize Redis
	InitRedis()
	defer RedisClient.Close()

	// Test data
	imsi := "001010123456789"
	quota := 1000 // 1000MB

	// Store quota in Redis
	err := RedisClient.HSet(RedisCtx, "quota:"+imsi,
		"remaining", quota,
		"updated", time.Now().Unix(),
	).Err()
	assert.NoError(t, err)

	// Retrieve quota
	val, err := RedisClient.HGet(RedisCtx, "quota:"+imsi, "remaining").Int()
	assert.NoError(t, err)
	assert.Equal(t, quota, val)

	// Update quota
	newQuota := 900
	err = RedisClient.HSet(RedisCtx, "quota:"+imsi, "remaining", newQuota).Err()
	assert.NoError(t, err)

	// Verify update
	val, err = RedisClient.HGet(RedisCtx, "quota:"+imsi, "remaining").Int()
	assert.NoError(t, err)
	assert.Equal(t, newQuota, val)
}

func TestQuotaDeduction(t *testing.T) {
	// Initialize Redis
	InitRedis()
	defer RedisClient.Close()

	// Test data
	imsi := "001010123456789"
	initialQuota := 1000
	deductAmount := 100

	// Set initial quota
	err := RedisClient.HSet(RedisCtx, "quota:"+imsi, "remaining", initialQuota).Err()
	assert.NoError(t, err)

	// Deduct quota
	newQuota := initialQuota - deductAmount
	err = RedisClient.HSet(RedisCtx, "quota:"+imsi, "remaining", newQuota).Err()
	assert.NoError(t, err)

	// Verify deduction
	val, err := RedisClient.HGet(RedisCtx, "quota:"+imsi, "remaining").Int()
	assert.NoError(t, err)
	assert.Equal(t, newQuota, val)
}

func TestQuotaExpiry(t *testing.T) {
	// Initialize Redis
	InitRedis()
	defer RedisClient.Close()

	// Test data
	imsi := "001010123456789"
	quota := 1000

	// Store quota with 1 second expiry
	err := RedisClient.HSet(RedisCtx, "quota:"+imsi, "remaining", quota).Err()
	assert.NoError(t, err)
	err = RedisClient.Expire(RedisCtx, "quota:"+imsi, 1*time.Second).Err()
	assert.NoError(t, err)

	// Wait for expiry
	time.Sleep(2 * time.Second)

	// Verify expiry
	exists, err := RedisClient.Exists(RedisCtx, "quota:"+imsi).Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(0), exists)
}
