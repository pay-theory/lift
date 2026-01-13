//go:build contract
// +build contract

// Package contracts implements hermetic contract tests for Lift CLI v1.
// These tests verify that the implementation matches docs/cli-contract-v1.md.
//
// Run with: go test -tags=contract ./internal/contracts
package contracts

import (
	"strings"
	"testing"

	"github.com/pay-theory/lift/internal/domains"
	"github.com/pay-theory/lift/internal/liftconfig"
	"github.com/pay-theory/lift/internal/liftstate"
	"github.com/pay-theory/lift/pkg/cli"
)

// TestStageContractParity verifies that the valid stages match the CLI contract.
// Contract source: docs/cli-contract-v1.md (Stages section)
// Implementation: internal/domains.ValidStages + internal/domains.ValidateStage
func TestStageContractParity(t *testing.T) {
	// Test A: ValidStages equals exactly: ["dev", "staging", "live"]
	expectedStages := []string{"dev", "staging", "live"}

	if len(domains.ValidStages) != len(expectedStages) {
		t.Fatalf("ValidStages length mismatch: got %d, want %d", len(domains.ValidStages), len(expectedStages))
	}

	for i, expected := range expectedStages {
		if domains.ValidStages[i] != expected {
			t.Errorf("ValidStages[%d] = %q, want %q", i, domains.ValidStages[i], expected)
		}
	}

	// Test B: ValidateStage accepts valid stages and rejects invalid ones
	for _, stage := range expectedStages {
		if err := domains.ValidateStage(stage); err != nil {
			t.Errorf("ValidateStage(%q) returned error: %v", stage, err)
		}
	}

	// Verify invalid stage is rejected
	invalidStage := "prod"
	if err := domains.ValidateStage(invalidStage); err == nil {
		t.Errorf("ValidateStage(%q) should have returned error, but got nil", invalidStage)
	} else {
		// Verify error message is informative
		errMsg := err.Error()
		if !strings.Contains(errMsg, "dev") || !strings.Contains(errMsg, "staging") || !strings.Contains(errMsg, "live") {
			t.Errorf("ValidateStage error message should mention valid stages, got: %s", errMsg)
		}
	}
}

// TestDomainDerivationContract verifies default domain derivation rules.
// Contract source: docs/cli-contract-v1.md (Domains section)
// Implementation: internal/domains.Resolve
func TestDomainDerivationContract(t *testing.T) {
	// Create in-memory config with base_domain and api service
	cfg := &liftconfig.Config{
		Domains: &liftconfig.Domains{
			BaseDomain: "example.com",
		},
		Services: map[string]*liftconfig.Service{
			"api": {
				Subdomain: "api",
			},
		},
		Stages: liftconfig.StageMap{
			"dev":     {},
			"staging": {},
			"live":    {},
		},
	}

	testCases := []struct {
		stage              string
		expectedStageRoot  string
		expectedAPIService string
	}{
		{
			stage:              "dev",
			expectedStageRoot:  "dev.example.com",
			expectedAPIService: "api.dev.example.com",
		},
		{
			stage:              "staging",
			expectedStageRoot:  "staging.example.com",
			expectedAPIService: "api.staging.example.com",
		},
		{
			stage:              "live",
			expectedStageRoot:  "example.com",
			expectedAPIService: "api.example.com",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.stage, func(t *testing.T) {
			resolved, err := domains.Resolve(cfg, tc.stage)
			if err != nil {
				t.Fatalf("Resolve failed for stage %q: %v", tc.stage, err)
			}

			if resolved == nil {
				t.Fatalf("Resolve returned nil for stage %q", tc.stage)
			}

			// Verify base domain
			if resolved.BaseDomain != cfg.Domains.BaseDomain {
				t.Errorf("BaseDomain = %q, want %q", resolved.BaseDomain, cfg.Domains.BaseDomain)
			}

			// Verify stage root domain
			if resolved.StageRootDomain != tc.expectedStageRoot {
				t.Errorf("StageRootDomain = %q, want %q", resolved.StageRootDomain, tc.expectedStageRoot)
			}

			// Verify service domain
			apiDomain, ok := resolved.Services["api"]
			if !ok {
				t.Fatalf("Services[\"api\"] not found in resolved domains")
			}

			if apiDomain != tc.expectedAPIService {
				t.Errorf("Services[\"api\"] = %q, want %q", apiDomain, tc.expectedAPIService)
			}
		})
	}
}

// TestDomainImmutabilityContract verifies domain locking behavior.
// Contract source: docs/cli-contract-v1.md (Domain Immutability section)
// Implementation: internal/liftstate.CheckDomainLock
func TestDomainImmutabilityContract(t *testing.T) {
	stage := "dev"

	// Create existing state (simulates previous deployment)
	existingState := &liftstate.StageState{
		Stage:           stage,
		BaseDomain:      "example.com",
		StageRootDomain: "dev.example.com",
		Services: map[string]liftstate.ServiceState{
			"api": {
				Subdomain: "api",
				Domain:    "api.dev.example.com",
			},
		},
		Version: 1,
	}

	// Test Case 1: Attempt to change baseDomain
	t.Run("changed_base_domain", func(t *testing.T) {
		newBaseDomain := "newexample.com"
		newStageRoot := "dev.newexample.com"
		services := map[string]string{"api": "api.dev.newexample.com"}

		err := liftstate.CheckDomainLock(existingState, stage, newBaseDomain, newStageRoot, services)
		if err == nil {
			t.Fatal("CheckDomainLock should return error when baseDomain changes, but got nil")
		}

		// Verify it's a DomainLockError
		lockErr, ok := err.(*liftstate.DomainLockError)
		if !ok {
			t.Fatalf("Expected *liftstate.DomainLockError, got %T: %v", err, err)
		}

		// Verify error message contains actionable guidance
		errMsg := lockErr.Error()
		expectedInstruction := "lift down --stage " + stage
		if !strings.Contains(errMsg, expectedInstruction) {
			t.Errorf("Error message should contain %q, got: %s", expectedInstruction, errMsg)
		}

		// Verify error mentions the stage
		if !strings.Contains(errMsg, stage) {
			t.Errorf("Error message should mention stage %q, got: %s", stage, errMsg)
		}
	})

	// Test Case 2: Attempt to change stageRootDomain
	t.Run("changed_stage_root_domain", func(t *testing.T) {
		newStageRoot := "development.example.com"
		services := map[string]string{"api": "api.development.example.com"}

		err := liftstate.CheckDomainLock(existingState, stage, existingState.BaseDomain, newStageRoot, services)
		if err == nil {
			t.Fatal("CheckDomainLock should return error when stageRootDomain changes, but got nil")
		}

		lockErr, ok := err.(*liftstate.DomainLockError)
		if !ok {
			t.Fatalf("Expected *liftstate.DomainLockError, got %T", err)
		}

		// Verify actionable guidance present
		if !strings.Contains(lockErr.Error(), "lift down") {
			t.Errorf("Error message should contain 'lift down' instruction")
		}
	})

	// Test Case 3: Attempt to change service domain
	t.Run("changed_service_domain", func(t *testing.T) {
		services := map[string]string{"api": "api2.dev.example.com"}

		err := liftstate.CheckDomainLock(existingState, stage, existingState.BaseDomain, existingState.StageRootDomain, services)
		if err == nil {
			t.Fatal("CheckDomainLock should return error when service domain changes, but got nil")
		}

		lockErr, ok := err.(*liftstate.DomainLockError)
		if !ok {
			t.Fatalf("Expected *liftstate.DomainLockError, got %T", err)
		}

		if len(lockErr.Mismatches) == 0 {
			t.Error("DomainLockError should have Mismatches describing the change")
		}
	})

	// Test Case 4: No changes - should pass
	t.Run("no_changes", func(t *testing.T) {
		services := map[string]string{"api": "api.dev.example.com"}

		err := liftstate.CheckDomainLock(existingState, stage, existingState.BaseDomain, existingState.StageRootDomain, services)
		if err != nil {
			t.Errorf("CheckDomainLock should return nil when domains unchanged, got: %v", err)
		}
	})

	// Test Case 5: No existing state - should pass
	t.Run("no_existing_state", func(t *testing.T) {
		services := map[string]string{"api": "api.dev.example.com"}

		err := liftstate.CheckDomainLock(nil, stage, "example.com", "dev.example.com", services)
		if err != nil {
			t.Errorf("CheckDomainLock should return nil when no existing state, got: %v", err)
		}
	})
}

// TestCLISurfaceContract verifies that CLI help text references canonical stages.
// Contract source: docs/cli-contract-v1.md (Stages section)
// Implementation: pkg/cli.UpCommand.Usage()
func TestCLISurfaceContract(t *testing.T) {
	upCmd := &cli.UpCommand{}
	usage := upCmd.Usage()

	// Verify usage mentions all three canonical stages
	canonicalStages := []string{"dev", "staging", "live"}
	for _, stage := range canonicalStages {
		if !strings.Contains(usage, stage) {
			t.Errorf("UpCommand.Usage() should reference stage %q, but it's missing.\nUsage:\n%s", stage, usage)
		}
	}

	// Verify usage describes --stage flag
	if !strings.Contains(usage, "--stage") {
		t.Error("UpCommand.Usage() should document --stage flag")
	}

	// Verify usage is non-empty
	if usage == "" {
		t.Fatal("UpCommand.Usage() returned empty string")
	}
}
