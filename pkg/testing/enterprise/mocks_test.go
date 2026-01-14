package enterprise

import "testing"

func TestServiceMockRegistry_SetupAndCleanup(t *testing.T) {
	t.Parallel()

	registry := NewServiceMockRegistry()
	env := &TestEnvironment{
		Name:      "dev",
		Resources: make(map[string]any),
		Config: EnvironmentConfig{
			ServiceEndpoints: nil,
		},
	}

	if err := registry.SetupForEnvironment(env); err != nil {
		t.Fatalf("SetupForEnvironment (no mocks) error: %v", err)
	}
	if err := registry.CleanupForEnvironment(env); err != nil {
		t.Fatalf("CleanupForEnvironment (no mocks) error: %v", err)
	}

	registry.AddMock("dev", ServiceMock{
		Name:     "payments",
		Endpoint: "http://mock-payments",
		Type:     "http",
		Active:   true,
	})

	if err := registry.SetupForEnvironment(env); err != nil {
		t.Fatalf("SetupForEnvironment error: %v", err)
	}

	if _, ok := env.Resources["mock_payments"]; !ok {
		t.Fatalf("mock_payments not stored in environment resources")
	}
	if env.Config.ServiceEndpoints == nil {
		t.Fatalf("ServiceEndpoints map not initialized")
	}
	if got := env.Config.ServiceEndpoints["payments"]; got != "http://mock-payments" {
		t.Fatalf("ServiceEndpoints[payments] = %q, want %q", got, "http://mock-payments")
	}

	if err := registry.CleanupForEnvironment(env); err != nil {
		t.Fatalf("CleanupForEnvironment error: %v", err)
	}
	if _, ok := env.Resources["mock_payments"]; ok {
		t.Fatalf("mock_payments still present after cleanup")
	}
}
