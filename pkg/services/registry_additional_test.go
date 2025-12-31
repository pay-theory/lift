package services

import (
	"context"
	"errors"
	"testing"

	"github.com/pay-theory/lift/pkg/lift/health"
	"github.com/stretchr/testify/require"
)

func TestServiceRegistry_DiscoverAllWatchListServices(t *testing.T) {
	discovery := newFakeDiscovery()
	discovery.setDiscoverResponse("svc", []*ServiceInstance{
		{ID: "a", ServiceName: "svc", Version: "v1", Health: HealthStatus{Status: "healthy"}},
		{ID: "b", ServiceName: "svc", Version: "v2", Health: HealthStatus{Status: "healthy"}},
		{ID: "c", ServiceName: "svc", Version: "v1", Health: HealthStatus{Status: "unhealthy"}},
	})

	registry := NewServiceRegistry(RegistryConfig{EnableMetrics: false}, discovery, newFakeLoadBalancer(nil))

	all, err := registry.DiscoverAll(context.Background(), "svc", DiscoveryOptions{Version: "v1"})
	require.NoError(t, err)
	require.Len(t, all, 1)
	require.Equal(t, "a", all[0].ID)

	discovery.discoverErr = errors.New("boom")
	_, err = registry.DiscoverAll(context.Background(), "svc", DiscoveryOptions{})
	require.Error(t, err)

	ch := make(chan []*ServiceInstance)
	discovery.watchCh = ch
	gotCh, err := registry.Watch(context.Background(), "svc")
	require.NoError(t, err)
	require.Equal(t, (<-chan []*ServiceInstance)(ch), gotCh)

	registry.services["svc-a"] = &ServiceConfig{Name: "svc-a"}
	registry.services["svc-b"] = &ServiceConfig{Name: "svc-b"}
	services := registry.ListServices()
	require.Len(t, services, 2)
}

func TestServiceRegistry_Register_StartsHealthMonitoringWhenEnabled(t *testing.T) {
	discovery := newFakeDiscovery()
	registry := NewServiceRegistry(RegistryConfig{EnableMetrics: false}, discovery, newFakeLoadBalancer(nil))
	registry.healthChecker = health.NewHealthManager(health.HealthManagerConfig{})

	cfg := &ServiceConfig{
		Name:    "orders",
		Version: "v1",
		Endpoints: []ServiceEndpoint{
			{Host: "localhost", Port: 8080, Protocol: "http"},
		},
		HealthCheck: HealthCheckConfig{
			Enabled: true,
		},
	}

	require.NoError(t, registry.Register(context.Background(), cfg))
}
