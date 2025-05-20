package integration_test

import (
	"context"
	"fmt"
	"time"

	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// ContainerManager manages test containers
type ContainerManager struct {
	ctx   context.Context
	redis tc.Container
	nats  tc.Container
	amf   tc.Container
	smf   tc.Container
	ocs   tc.Container
	bss   tc.Container
	ports map[string]string
}

// NewContainerManager creates a new container manager
func NewContainerManager(ctx context.Context) *ContainerManager {
	return &ContainerManager{
		ctx:   ctx,
		ports: make(map[string]string),
	}
}

// StartInfrastructure starts Redis and NATS
func (cm *ContainerManager) StartInfrastructure() error {
	// Start Redis
	redis, err := tc.GenericContainer(cm.ctx, tc.GenericContainerRequest{
		ContainerRequest: tc.ContainerRequest{
			Image:        "redis:7-alpine",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForListeningPort("6379/tcp"),
		},
		Started: true,
	})
	if err != nil {
		return fmt.Errorf("failed to start Redis: %v", err)
	}
	cm.redis = redis

	// Start NATS
	nats, err := tc.GenericContainer(cm.ctx, tc.GenericContainerRequest{
		ContainerRequest: tc.ContainerRequest{
			Image:        "nats:2.10.11-alpine",
			ExposedPorts: []string{"4222/tcp"},
			Cmd:          []string{"-js"},
			WaitingFor:   wait.ForListeningPort("4222/tcp"),
		},
		Started: true,
	})
	if err != nil {
		return fmt.Errorf("failed to start NATS: %v", err)
	}
	cm.nats = nats

	// Get connection details
	redisHost, err := redis.Host(cm.ctx)
	if err != nil {
		return fmt.Errorf("failed to get Redis host: %v", err)
	}
	redisPort, err := redis.MappedPort(cm.ctx, "6379")
	if err != nil {
		return fmt.Errorf("failed to get Redis port: %v", err)
	}

	natsHost, err := nats.Host(cm.ctx)
	if err != nil {
		return fmt.Errorf("failed to get NATS host: %v", err)
	}
	natsPort, err := nats.MappedPort(cm.ctx, "4222")
	if err != nil {
		return fmt.Errorf("failed to get NATS port: %v", err)
	}

	// Store connection details
	cm.ports["redis"] = fmt.Sprintf("%s:%s", redisHost, redisPort.Port())
	cm.ports["nats"] = fmt.Sprintf("%s:%s", natsHost, natsPort.Port())

	return nil
}

// StartService starts a specific service
func (cm *ContainerManager) StartService(service string) error {
	var container tc.Container
	var err error

	switch service {
	case "amf":
		container, err = tc.GenericContainer(cm.ctx, tc.GenericContainerRequest{
			ContainerRequest: tc.ContainerRequest{
				Image:        "openmvcore_amf",
				ExposedPorts: []string{"8081/tcp"},
				Env: map[string]string{
					"REDIS_ADDR": cm.ports["redis"],
					"NATS_URL":   fmt.Sprintf("nats://%s", cm.ports["nats"]),
				},
				WaitingFor: wait.ForHTTP("/health").WithPort("8081/tcp").WithStartupTimeout(30 * time.Second),
			},
			Started: true,
		})
	case "smf":
		container, err = tc.GenericContainer(cm.ctx, tc.GenericContainerRequest{
			ContainerRequest: tc.ContainerRequest{
				Image:        "openmvcore_smf",
				ExposedPorts: []string{"8805/tcp"},
				Env: map[string]string{
					"REDIS_ADDR": cm.ports["redis"],
					"NATS_URL":   fmt.Sprintf("nats://%s", cm.ports["nats"]),
				},
				WaitingFor: wait.ForHTTP("/health").WithPort("8805/tcp").WithStartupTimeout(30 * time.Second),
			},
			Started: true,
		})
	case "ocs":
		container, err = tc.GenericContainer(cm.ctx, tc.GenericContainerRequest{
			ContainerRequest: tc.ContainerRequest{
				Image:        "openmvcore_ocs",
				ExposedPorts: []string{"8082/tcp"},
				Env: map[string]string{
					"REDIS_ADDR": cm.ports["redis"],
					"NATS_URL":   fmt.Sprintf("nats://%s", cm.ports["nats"]),
				},
				WaitingFor: wait.ForHTTP("/health").WithPort("8082/tcp").WithStartupTimeout(30 * time.Second),
			},
			Started: true,
		})
	case "bss":
		container, err = tc.GenericContainer(cm.ctx, tc.GenericContainerRequest{
			ContainerRequest: tc.ContainerRequest{
				Image:        "openmvcore_bss",
				ExposedPorts: []string{"8084/tcp"},
				Env: map[string]string{
					"REDIS_ADDR": cm.ports["redis"],
					"NATS_URL":   fmt.Sprintf("nats://%s", cm.ports["nats"]),
				},
				WaitingFor: wait.ForHTTP("/health").WithPort("8084/tcp").WithStartupTimeout(30 * time.Second),
			},
			Started: true,
		})
	default:
		return fmt.Errorf("unknown service: %s", service)
	}

	if err != nil {
		return fmt.Errorf("failed to start %s: %v", service, err)
	}

	// Store container and get port
	host, err := container.Host(cm.ctx)
	if err != nil {
		return fmt.Errorf("failed to get %s host: %v", service, err)
	}

	var portStr string
	switch service {
	case "amf":
		port, err := container.MappedPort(cm.ctx, "8081")
		if err != nil {
			return fmt.Errorf("failed to get AMF port: %v", err)
		}
		portStr = port.Port()
		cm.amf = container
	case "smf":
		port, err := container.MappedPort(cm.ctx, "8805")
		if err != nil {
			return fmt.Errorf("failed to get SMF port: %v", err)
		}
		portStr = port.Port()
		cm.smf = container
	case "ocs":
		port, err := container.MappedPort(cm.ctx, "8082")
		if err != nil {
			return fmt.Errorf("failed to get OCS port: %v", err)
		}
		portStr = port.Port()
		cm.ocs = container
	case "bss":
		port, err := container.MappedPort(cm.ctx, "8084")
		if err != nil {
			return fmt.Errorf("failed to get BSS port: %v", err)
		}
		portStr = port.Port()
		cm.bss = container
	}

	cm.ports[service] = fmt.Sprintf("%s:%s", host, portStr)
	return nil
}

// GetServiceURL returns the URL for a service
func (cm *ContainerManager) GetServiceURL(service string) string {
	return fmt.Sprintf("http://%s", cm.ports[service])
}

// Cleanup terminates all containers
func (cm *ContainerManager) Cleanup() {
	containers := []tc.Container{cm.redis, cm.nats, cm.amf, cm.smf, cm.ocs, cm.bss}
	for _, c := range containers {
		if c != nil {
			c.Terminate(cm.ctx)
		}
	}
}
