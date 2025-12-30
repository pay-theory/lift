package enterprise

import (
	"errors"
	"strings"
	"testing"
)

type staticValidator struct {
	name string
	err  error
}

func (v staticValidator) Validate(_ *TestEnvironment) error { return v.err }
func (v staticValidator) Name() string                      { return v.name }

func TestEnterpriseTestSuite_AddGetSwitchReset(t *testing.T) {
	suite := NewEnterpriseTestSuite()

	dev := suite.AddEnvironment("dev", EnvironmentConfig{
		DatabaseURL: "db://local",
	})
	if dev.Name != "dev" {
		t.Fatalf("env.Name = %q, want %q", dev.Name, "dev")
	}

	if _, err := suite.GetEnvironment("missing"); err == nil {
		t.Fatalf("GetEnvironment(missing) expected error")
	}

	if err := suite.SwitchEnvironment("dev"); err != nil {
		t.Fatalf("SwitchEnvironment(dev) error: %v", err)
	}

	dev.mutex.Lock()
	dev.State.Status = EnvironmentStatusBusy
	dev.mutex.Unlock()

	if err := suite.SwitchEnvironment("dev"); err == nil {
		t.Fatalf("SwitchEnvironment(dev) expected error when busy")
	}

	dev.Resources["k"] = "v"
	dev.State.Metrics.RequestCount = 5
	if err := suite.ResetEnvironment("dev"); err != nil {
		t.Fatalf("ResetEnvironment(dev) error: %v", err)
	}
	if len(dev.Resources) != 0 {
		t.Fatalf("Resources not reset: %#v", dev.Resources)
	}
	if dev.State.Metrics.RequestCount != 0 {
		t.Fatalf("RequestCount not reset: %d", dev.State.Metrics.RequestCount)
	}
}

func TestEnterpriseTestSuite_RunTestInEnvironment_CleansUpAndUpdatesMetrics(t *testing.T) {
	suite := NewEnterpriseTestSuite()

	env := suite.AddEnvironment("dev", EnvironmentConfig{
		DatabaseURL: "db://local",
	})

	suite.dataFixtures.AddFixture("dev", DataFixture{Name: "users", Data: "u"})
	suite.mockServices.AddMock("dev", ServiceMock{
		Name:     "payments",
		Endpoint: "http://mock-payments",
	})

	env.Validators = []EnvironmentValidator{
		&DatabaseValidator{},
		&ServiceValidator{},
		&ResourceValidator{},
	}

	testCase := TestCase{
		Name: "basic",
		Execute: func(app *EnterpriseTestApp, testEnv *TestEnvironment) error {
			if app.environment != testEnv {
				return errors.New("test app environment mismatch")
			}
			if got := testEnv.Resources["fixture_users"]; got != "u" {
				return errors.New("missing fixture_users in environment resources")
			}
			if _, ok := testEnv.Resources["mock_payments"]; !ok {
				return errors.New("missing mock_payments in environment resources")
			}
			return nil
		},
	}

	if err := suite.runTestInEnvironment(testCase, "dev", env); err != nil {
		t.Fatalf("runTestInEnvironment error: %v", err)
	}

	if env.State.Status != EnvironmentStatusReady {
		t.Fatalf("env.State.Status = %q, want %q", env.State.Status, EnvironmentStatusReady)
	}
	if env.State.ActiveTests != 0 {
		t.Fatalf("env.State.ActiveTests = %d, want 0", env.State.ActiveTests)
	}
	if env.State.Metrics.RequestCount != 1 {
		t.Fatalf("RequestCount = %d, want 1", env.State.Metrics.RequestCount)
	}
	if env.State.Metrics.ErrorCount != 0 {
		t.Fatalf("ErrorCount = %d, want 0", env.State.Metrics.ErrorCount)
	}
	if _, ok := env.Resources["fixture_users"]; ok {
		t.Fatalf("fixture_users still present after cleanup")
	}
	if _, ok := env.Resources["mock_payments"]; ok {
		t.Fatalf("mock_payments still present after cleanup")
	}

	metrics, err := suite.GetEnvironmentMetrics("dev")
	if err != nil {
		t.Fatalf("GetEnvironmentMetrics error: %v", err)
	}
	if metrics.RequestCount != 1 {
		t.Fatalf("GetEnvironmentMetrics.RequestCount=%d, want 1", metrics.RequestCount)
	}
}

func TestEnterpriseTestSuite_RunTestInEnvironment_ValidationFailure(t *testing.T) {
	suite := NewEnterpriseTestSuite()

	env := suite.AddEnvironment("dev", EnvironmentConfig{
		DatabaseURL: "",
	})
	env.Validators = []EnvironmentValidator{
		&DatabaseValidator{},
	}

	err := suite.runTestInEnvironment(TestCase{
		Name:    "noop",
		Execute: func(_ *EnterpriseTestApp, _ *TestEnvironment) error { return nil },
	}, "dev", env)
	if err == nil {
		t.Fatalf("runTestInEnvironment expected validation error")
	}
	if !strings.Contains(err.Error(), "environment validation failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnterpriseTestSuite_TestAcrossEnvironments_AggregatesErrors(t *testing.T) {
	suite := NewEnterpriseTestSuite()

	good := suite.AddEnvironment("good", EnvironmentConfig{DatabaseURL: "db://ok"})
	good.Validators = []EnvironmentValidator{
		staticValidator{name: "ok", err: nil},
	}

	bad := suite.AddEnvironment("bad", EnvironmentConfig{DatabaseURL: "db://ok"})
	bad.Validators = []EnvironmentValidator{
		staticValidator{name: "fail", err: errors.New("boom")},
	}

	err := suite.TestAcrossEnvironments(TestCase{
		Name:    "noop",
		Execute: func(_ *EnterpriseTestApp, _ *TestEnvironment) error { return nil },
	})
	if err == nil {
		t.Fatalf("TestAcrossEnvironments expected error")
	}
	if !strings.Contains(err.Error(), "test failures in 1 environments") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "environment bad") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnvironmentValidator_Names(t *testing.T) {
	if (&DatabaseValidator{}).Name() != "database" {
		t.Fatalf("DatabaseValidator.Name unexpected")
	}
	if (&ServiceValidator{}).Name() != "services" {
		t.Fatalf("ServiceValidator.Name unexpected")
	}
	if (&ResourceValidator{}).Name() != "resources" {
		t.Fatalf("ResourceValidator.Name unexpected")
	}
}
