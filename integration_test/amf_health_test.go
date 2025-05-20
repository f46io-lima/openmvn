package integration_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAMFHealthEndpoint(t *testing.T) {
	ctx := context.Background()
	cm := NewContainerManager(ctx)
	defer cm.Cleanup()

	// Start infrastructure
	err := cm.StartInfrastructure()
	require.NoError(t, err)

	// Start AMF
	err = cm.StartService("amf")
	require.NoError(t, err)

	// Give AMF time to initialize
	time.Sleep(3 * time.Second)

	// Check health endpoint
	resp, err := http.Get(cm.GetServiceURL("amf") + "/health")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestServiceIntegration(t *testing.T) {
	ctx := context.Background()
	cm := NewContainerManager(ctx)
	defer cm.Cleanup()

	// Start infrastructure
	err := cm.StartInfrastructure()
	require.NoError(t, err)

	// Test each service
	services := []string{"amf", "smf", "ocs", "bss"}
	for _, service := range services {
		t.Run(service, func(t *testing.T) {
			err := cm.StartService(service)
			require.NoError(t, err)

			// Give service time to initialize
			time.Sleep(3 * time.Second)

			// Check health endpoint
			resp, err := http.Get(cm.GetServiceURL(service) + "/health")
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
		})
	}
}
