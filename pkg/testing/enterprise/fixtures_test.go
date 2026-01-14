package enterprise

import "testing"

func TestDataFixtureManager_SetupAndCleanup(t *testing.T) {
	t.Parallel()

	manager := NewDataFixtureManager()
	env := &TestEnvironment{
		Name:      "dev",
		Resources: make(map[string]any),
	}

	if err := manager.SetupForEnvironment(env); err != nil {
		t.Fatalf("SetupForEnvironment (no fixtures) error: %v", err)
	}
	if err := manager.CleanupForEnvironment(env); err != nil {
		t.Fatalf("CleanupForEnvironment (no fixtures) error: %v", err)
	}

	manager.AddFixture("dev", DataFixture{Name: "users", Data: "u"})
	manager.AddFixture("dev", DataFixture{Name: "orders", Data: 123})

	if err := manager.SetupForEnvironment(env); err != nil {
		t.Fatalf("SetupForEnvironment error: %v", err)
	}
	if got := env.Resources["fixture_users"]; got != "u" {
		t.Fatalf("fixture_users = %#v, want %q", got, "u")
	}
	if got := env.Resources["fixture_orders"]; got != 123 {
		t.Fatalf("fixture_orders = %#v, want %d", got, 123)
	}

	if err := manager.CleanupForEnvironment(env); err != nil {
		t.Fatalf("CleanupForEnvironment error: %v", err)
	}
	if _, ok := env.Resources["fixture_users"]; ok {
		t.Fatalf("fixture_users still present after cleanup")
	}
	if _, ok := env.Resources["fixture_orders"]; ok {
		t.Fatalf("fixture_orders still present after cleanup")
	}
}
