//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestIMSIReceiverIntegration(t *testing.T) {
	ctx := context.Background()

	// Start NATS container
	natsContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "nats:2.10.11-alpine",
			ExposedPorts: []string{"4222/tcp"},
			Cmd:          []string{"-js"},
			WaitingFor:   wait.ForListeningPort("4222/tcp"),
		},
		Started: true,
	})
	require.NoError(t, err)
	defer natsContainer.Terminate(ctx)

	// Get NATS connection details
	natsHost, err := natsContainer.Host(ctx)
	require.NoError(t, err)
	natsPort, err := natsContainer.MappedPort(ctx, "4222")
	require.NoError(t, err)

	// Connect to NATS
	nc, err := nats.Connect("nats://" + natsHost + ":" + natsPort.Port())
	require.NoError(t, err)
	defer nc.Close()

	// Start the IMSI receiver service
	go main()                   // Start the service in a goroutine
	time.Sleep(2 * time.Second) // Wait for service to start

	// Test cases
	t.Run("Switch Decision Flow", func(t *testing.T) {
		// 1. Publish a switch decision
		js, err := nc.JetStream()
		require.NoError(t, err)

		event := SwitchDecision{
			IMSI:     "001010123456789",
			OldIMSI:  "001010000000000",
			DeviceID: "test-device-1",
			Reason:   "test switch",
			Time:     time.Now(),
			Status:   "pending",
		}

		data, err := json.Marshal(event)
		require.NoError(t, err)

		_, err = js.Publish("imsi.switch", data)
		require.NoError(t, err)

		// 2. Verify the switch decision was received
		time.Sleep(1 * time.Second) // Wait for processing

		resp, err := http.Get("http://localhost:8080/switches/test-device-1")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var received SwitchDecision
		err = json.NewDecoder(resp.Body).Decode(&received)
		require.NoError(t, err)
		assert.Equal(t, event.IMSI, received.IMSI)
		assert.Equal(t, event.Status, received.Status)

		// 3. Accept the switch
		acceptData, err := json.Marshal(map[string]string{"status": "accepted"})
		require.NoError(t, err)

		resp, err = http.Post(
			"http://localhost:8080/switches/test-device-1/respond",
			"application/json",
			bytes.NewBuffer(acceptData),
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// 4. Verify the switch was removed after acceptance
		time.Sleep(1 * time.Second)
		resp, err = http.Get("http://localhost:8080/switches/test-device-1")
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("Health Check", func(t *testing.T) {
		resp, err := http.Get("http://localhost:8080/health")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, "OK", string(body))
	})
}
