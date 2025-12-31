package enterprise

import (
	"context"
	"errors"
	"testing"
	"time"
)

type failValidator struct{}

func (failValidator) ValidateContract(_ context.Context, _ *ServiceContract) (*TestResult, error) {
	return nil, errors.New("validate failed")
}

func (failValidator) ValidateInteraction(_ context.Context, _ *ContractInteraction) (*InteractionResult, error) {
	return nil, errors.New("validate interaction failed")
}

func TestBasicContractValidator_Validate(t *testing.T) {
	validator := NewBasicContractValidator()
	ctx := context.Background()

	if err := validator.Validate(ctx, &ContractInteraction{
		ID:       "i1",
		Request:  nil,
		Response: &InteractionResponse{Status: 200},
	}); err == nil {
		t.Fatalf("expected request validation error")
	}

	if err := validator.Validate(ctx, &ContractInteraction{
		ID:      "i2",
		Request: &InteractionRequest{Method: "GET", Path: "/"},
	}); err == nil {
		t.Fatalf("expected response validation error")
	}

	if err := validator.Validate(ctx, &ContractInteraction{
		ID:      "i3",
		Request: &InteractionRequest{Method: "GET", Path: "/"},
		Response: &InteractionResponse{
			Status: 99,
		},
	}); err == nil {
		t.Fatalf("expected invalid status error")
	}

	if err := validator.Validate(ctx, &ContractInteraction{
		ID:      "i4",
		Request: &InteractionRequest{Method: "GET", Path: "/"},
		Response: &InteractionResponse{
			Status: 200,
		},
	}); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if validator.metrics.TotalValidations != 4 {
		t.Fatalf("TotalValidations=%d, want 4", validator.metrics.TotalValidations)
	}
	if validator.metrics.SuccessfulValidations == 0 {
		t.Fatalf("expected at least one successful validation")
	}
	if validator.metrics.FailedValidations == 0 {
		t.Fatalf("expected at least one failed validation")
	}
}

func TestBasicContractValidator_ValidateContractAndInteraction(t *testing.T) {
	validator := NewBasicContractValidator()

	if _, err := validator.ValidateContract(context.Background(), nil); err == nil {
		t.Fatalf("ValidateContract(nil) expected error")
	}

	contract := &ServiceContract{
		ID:   "c1",
		Name: "svc",
		Provider: ServiceInfo{
			Name: "provider",
		},
		Consumer: ServiceInfo{
			Name: "consumer",
		},
		Interactions: []ContractInteraction{
			{
				ID:      "i1",
				Request: &InteractionRequest{Method: "", Path: "/"},
				Response: &InteractionResponse{
					Status: 200,
				},
			},
		},
	}

	if _, err := validator.ValidateContract(context.Background(), contract); err == nil {
		t.Fatalf("ValidateContract expected error for invalid interaction")
	}

	interaction := &ContractInteraction{
		ID:      "i2",
		Request: &InteractionRequest{Method: "GET", Path: "/"},
		Response: &InteractionResponse{
			Status: 700,
		},
	}
	if _, err := validator.ValidateInteraction(context.Background(), interaction); err == nil {
		t.Fatalf("ValidateInteraction expected error")
	}
}

func TestContractTestingFramework_ValidationHelpers(t *testing.T) {
	framework := NewContractTestingFramework(nil)

	contract := &ServiceContract{
		ID:   "c1",
		Name: "svc",
		Provider: ServiceInfo{
			Name: "provider",
		},
		Consumer: ServiceInfo{
			Name: "consumer",
		},
		Interactions: []ContractInteraction{
			{
				ID: "ok",
				Request: &InteractionRequest{
					Method:  "GET",
					Path:    "/",
					Headers: map[string]string{"x": "y"},
				},
				Response: &InteractionResponse{
					Status:  200,
					Headers: map[string]string{"x": "y"},
				},
			},
			{
				ID: "bad",
				Request: &InteractionRequest{
					Method:  "BAD",
					Path:    "missing_slash",
					Headers: map[string]string{"": ""},
				},
				Response: &InteractionResponse{
					Status:  200,
					Headers: map[string]string{"": ""},
				},
			},
		},
	}

	result, err := framework.ValidateContract(context.Background(), contract)
	if err != nil {
		t.Fatalf("ValidateContract error: %v", err)
	}
	if result.Status != TestStatusFailed {
		t.Fatalf("Status=%q, want %q", result.Status, TestStatusFailed)
	}

	if got := framework.validateHTTPMethod("GET"); !got.Valid {
		t.Fatalf("validateHTTPMethod(GET) expected valid")
	}
	if got := framework.validateHTTPMethod("NOPE"); got.Valid {
		t.Fatalf("validateHTTPMethod(NOPE) expected invalid")
	}

	if got := framework.validateHTTPPath("/"); !got.Valid {
		t.Fatalf("validateHTTPPath(/) expected valid")
	}
	if got := framework.validateHTTPPath(""); got.Valid {
		t.Fatalf("validateHTTPPath(empty) expected invalid")
	}
	if got := framework.validateHTTPPath("nope"); got.Valid {
		t.Fatalf("validateHTTPPath(nope) expected invalid")
	}

	if got := framework.validateHeaders(map[string]string{"": ""}); got.Valid {
		t.Fatalf("validateHeaders expected invalid")
	}
}

func TestContractTestingFramework_SchemaAndStatusHelpers(t *testing.T) {
	framework := NewContractTestingFramework(nil)

	check, err := framework.validateSchema("x", nil)
	if err != nil {
		t.Fatalf("validateSchema(nil) error: %v", err)
	}
	if check.Valid {
		t.Fatalf("validateSchema(nil) expected invalid")
	}

	minLen := 2
	maxLen := 3
	check, err = framework.validateSchema("x", &SchemaDefinition{Type: "string", MinLength: &minLen, MaxLength: &maxLen})
	if err != nil {
		t.Fatalf("validateSchema(string) error: %v", err)
	}
	if check.Valid {
		t.Fatalf("validateSchema(string) expected invalid due to length")
	}

	min := 10.0
	max := 20.0
	check, err = framework.validateSchema(5.0, &SchemaDefinition{Type: "number", Minimum: &min, Maximum: &max})
	if err != nil {
		t.Fatalf("validateSchema(number) error: %v", err)
	}
	if check.Valid {
		t.Fatalf("validateSchema(number) expected invalid due to minimum")
	}

	check, err = framework.validateSchema(map[string]any{"a": 1}, &SchemaDefinition{Type: "object", Required: []string{"missing"}})
	if err != nil {
		t.Fatalf("validateSchema(object) error: %v", err)
	}
	if check.Valid {
		t.Fatalf("validateSchema(object) expected invalid due to missing required")
	}

	if err := framework.validateType("x", "string"); err != nil {
		t.Fatalf("validateType string error: %v", err)
	}
	if err := framework.validateType(1, "number"); err != nil {
		t.Fatalf("validateType number error: %v", err)
	}
	if err := framework.validateType(true, "boolean"); err != nil {
		t.Fatalf("validateType boolean error: %v", err)
	}
	if err := framework.validateType(map[string]any{}, "object"); err != nil {
		t.Fatalf("validateType object error: %v", err)
	}
	if err := framework.validateType([]any{}, "array"); err != nil {
		t.Fatalf("validateType array error: %v", err)
	}
	if err := framework.validateType(123, "unknown"); err == nil {
		t.Fatalf("validateType unknown expected error")
	}

	summary := framework.generateValidationSummary([]*ContractValidationResult{
		{Status: TestStatusPassed},
		{Status: TestStatusFailed},
	})
	if summary == "" {
		t.Fatalf("generateValidationSummary returned empty string")
	}

	if got := framework.calculateValidationStatus(nil); got != TestStatus("unknown") {
		t.Fatalf("calculateValidationStatus(nil)=%q, want %q", got, TestStatus("unknown"))
	}
	if got := framework.calculateValidationStatus(map[string]*InteractionValidation{
		"x": {Status: string(JobFailed)},
	}); got != TestStatusFailed {
		t.Fatalf("calculateValidationStatus failed=%q, want %q", got, TestStatusFailed)
	}

	if got := framework.calculateInteractionStatus(nil); got != TestStatus("unknown") {
		t.Fatalf("calculateInteractionStatus(nil)=%q, want %q", got, TestStatus("unknown"))
	}
	if got := framework.calculateInteractionStatus(map[string]*ValidationCheck{
		"x": {Status: string(JobFailed)},
	}); got != TestStatusFailed {
		t.Fatalf("calculateInteractionStatus failed=%q, want %q", got, TestStatusFailed)
	}
}

func TestContractTestingFramework_RunContractTest(t *testing.T) {
	framework := NewContractTestingFramework(nil)

	contract := &ServiceContract{
		ID:   "c1",
		Name: "svc",
		Provider: ServiceInfo{
			Name: "provider",
		},
		Consumer: ServiceInfo{
			Name: "consumer",
		},
		Interactions: []ContractInteraction{
			{
				ID:      "i1",
				Request: &InteractionRequest{Method: "GET", Path: "/"},
				Response: &InteractionResponse{
					Status: 200,
				},
			},
		},
	}

	test, err := framework.CreateContractTest(contract, framework.validator)
	if err != nil {
		t.Fatalf("CreateContractTest error: %v", err)
	}

	// Force validator error while keeping contract non-nil (RunContractTest reads contract fields on error).
	test.Validator = failValidator{}
	result, err := framework.RunContractTest(context.Background(), test)
	if err == nil || result == nil || result.Status != TestStatusFailed {
		t.Fatalf("RunContractTest expected failure result, got result=%#v err=%v", result, err)
	}
}

func TestContractTestingFramework_CreateContractTest_Errors(t *testing.T) {
	framework := NewContractTestingFramework(nil)

	if _, err := framework.CreateContractTest(nil, nil); err == nil {
		t.Fatalf("CreateContractTest(nil) expected error")
	}

	contract := &ServiceContract{
		ID:   "c1",
		Name: "svc",
		Provider: ServiceInfo{
			Name: "provider",
		},
		Consumer: ServiceInfo{
			Name: "consumer",
		},
	}
	test, err := framework.CreateContractTest(contract, nil)
	if err != nil {
		t.Fatalf("CreateContractTest error: %v", err)
	}
	if test.Validator == nil {
		t.Fatalf("expected default validator to be set")
	}
}

func TestContractTestRunnerAndRegistry_Constructors(t *testing.T) {
	runner := NewContractTestRunner(nil)
	if runner == nil || runner.registry == nil || runner.config == nil {
		t.Fatalf("expected runner and dependencies to be initialized")
	}

	registry := NewContractRegistry(nil)
	if registry == nil {
		t.Fatalf("expected registry")
	}

	reporter := NewContractTestReporter()
	if reporter == nil || reporter.templates == nil || reporter.exporters == nil {
		t.Fatalf("expected reporter to be initialized")
	}

	// Ensure metrics update doesn't panic when called multiple times.
	v := NewBasicContractValidator()
	before := v.metrics.LastValidation
	v.updateMetrics(1 * time.Millisecond)
	if !v.metrics.LastValidation.After(before) && !v.metrics.LastValidation.Equal(before) {
		t.Fatalf("expected LastValidation to be updated")
	}
}

func TestContractTestingFramework_ValidateType_Errors(t *testing.T) {
	framework := NewContractTestingFramework(nil)
	if err := framework.validateType(struct{}{}, "string"); err == nil {
		t.Fatalf("validateType string expected error")
	}
	if err := framework.validateType(struct{}{}, "number"); err == nil {
		t.Fatalf("validateType number expected error")
	}
	if err := framework.validateType(struct{}{}, "boolean"); err == nil {
		t.Fatalf("validateType boolean expected error")
	}
	if err := framework.validateType(struct{}{}, "object"); err == nil {
		t.Fatalf("validateType object expected error")
	}
	if err := framework.validateType(struct{}{}, "array"); err == nil {
		t.Fatalf("validateType array expected error")
	}
}

func TestContractTestingFramework_ValidateContract_AppLevel(t *testing.T) {
	framework := NewContractTestingFramework(nil)

	contract := &ServiceContract{
		ID:   "c1",
		Name: "svc",
		Provider: ServiceInfo{
			Name: "provider",
		},
		Consumer: ServiceInfo{
			Name: "consumer",
		},
		Interactions: []ContractInteraction{
			{
				ID: "only",
				Request: &InteractionRequest{
					Method:  "GET",
					Path:    "/",
					Headers: map[string]string{"a": "b"},
				},
				Response: &InteractionResponse{
					Status:  200,
					Headers: map[string]string{"a": "b"},
				},
			},
		},
	}

	if _, err := framework.ValidateContract(context.Background(), contract); err != nil {
		t.Fatalf("ValidateContract expected nil error, got: %v", err)
	}

	if _, err := framework.RunContractTest(context.Background(), &ContractTest{
		Provider:  "p",
		Consumer:  "c",
		Contract:  contract,
		Validator: framework.validator,
	}); err != nil {
		t.Fatalf("RunContractTest expected success, got: %v", err)
	}
}
