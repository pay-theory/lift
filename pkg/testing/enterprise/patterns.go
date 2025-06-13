package enterprise

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// EnterpriseTestPatterns provides enterprise-grade testing patterns
type EnterpriseTestPatterns struct {
	contractSuite *ContractTestSuite
	chaosTest     *ChaosTest
	performance   *PerformanceValidator
	environments  map[string]*TestEnvironment
	mutex         sync.RWMutex
}

// NewEnterpriseTestPatterns creates a new enterprise test patterns instance
func NewEnterpriseTestPatterns() *EnterpriseTestPatterns {
	return &EnterpriseTestPatterns{
		contractSuite: NewContractTestSuite(),
		chaosTest: NewChaosTest(ChaosConfig{
			MaxDuration:     30 * time.Minute,
			RecoveryTimeout: 5 * time.Minute,
			FailureRate:     0.1,
			Enabled:         true,
		}),
		performance:  NewPerformanceValidator(),
		environments: make(map[string]*TestEnvironment),
	}
}

// TestSuite represents a comprehensive test suite
type TestSuite struct {
	Name        string
	Description string
	Tests       []Test
	Setup       func() error
	Teardown    func() error
	Parallel    bool
	Timeout     time.Duration
	Environment string
}

// Test represents a single test
type Test struct {
	Name        string
	Description string
	Function    func(ctx context.Context) error
	Timeout     time.Duration
	Retries     int
	Tags        []string
}

// PatternTestResult represents the result of running a pattern test
type PatternTestResult struct {
	Name      string
	Status    PatternTestStatus
	Duration  time.Duration
	Error     error
	StartTime time.Time
	EndTime   time.Time
	Metadata  map[string]interface{}
}

// PatternTestStatus is an alias for the common TestStatus type
type PatternTestStatus = TestStatus

// RunTestSuite runs a complete test suite
func (e *EnterpriseTestPatterns) RunTestSuite(ctx context.Context, suite TestSuite) ([]TestResult, error) {
	var results []TestResult

	// Setup
	if suite.Setup != nil {
		if err := suite.Setup(); err != nil {
			return nil, fmt.Errorf("test suite setup failed: %w", err)
		}
	}

	// Teardown
	defer func() {
		if suite.Teardown != nil {
			suite.Teardown()
		}
	}()

	// Run tests
	if suite.Parallel {
		results = e.runTestsParallel(ctx, suite.Tests)
	} else {
		results = e.runTestsSequential(ctx, suite.Tests)
	}

	return results, nil
}

// runTestsSequential runs tests sequentially
func (e *EnterpriseTestPatterns) runTestsSequential(ctx context.Context, tests []Test) []TestResult {
	var results []TestResult

	for _, test := range tests {
		result := e.runSingleTest(ctx, test)
		results = append(results, result)
	}

	return results
}

// runTestsParallel runs tests in parallel
func (e *EnterpriseTestPatterns) runTestsParallel(ctx context.Context, tests []Test) []TestResult {
	results := make([]TestResult, len(tests))
	var wg sync.WaitGroup

	for i, test := range tests {
		wg.Add(1)
		go func(index int, t Test) {
			defer wg.Done()
			results[index] = e.runSingleTest(ctx, t)
		}(i, test)
	}

	wg.Wait()
	return results
}

// runSingleTest runs a single test
func (e *EnterpriseTestPatterns) runSingleTest(ctx context.Context, test Test) TestResult {
	result := TestResult{
		Name:      test.Name,
		StartTime: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	// Set timeout
	testCtx := ctx
	if test.Timeout > 0 {
		var cancel context.CancelFunc
		testCtx, cancel = context.WithTimeout(ctx, test.Timeout)
		defer cancel()
	}

	// Run test with retries
	var err error
	for attempt := 0; attempt <= test.Retries; attempt++ {
		err = test.Function(testCtx)
		if err == nil {
			break
		}

		if attempt < test.Retries {
			time.Sleep(time.Duration(attempt+1) * time.Second)
		}
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	if err != nil {
		result.Status = TestStatusFailed
		result.Error = err
	} else {
		result.Status = TestStatusPassed
	}

	return result
}

// RunContractTests runs contract tests
func (e *EnterpriseTestPatterns) RunContractTests(ctx context.Context) (map[string]ContractTestResult, error) {
	return e.contractSuite.RunContractTests()
}

// RunChaosTests runs chaos engineering tests
func (e *EnterpriseTestPatterns) RunChaosTests(ctx context.Context) error {
	return e.chaosTest.ExecuteChaos(ctx)
}

// ValidatePerformance validates performance across environments
func (e *EnterpriseTestPatterns) ValidatePerformance(ctx context.Context, testCase TestCase, envName string) error {
	env, exists := e.environments[envName]
	if !exists {
		return fmt.Errorf("environment %s not found", envName)
	}

	return e.performance.ValidatePerformance(testCase, env)
}

// AddEnvironment adds a test environment
func (e *EnterpriseTestPatterns) AddEnvironment(name string, config EnvironmentConfig) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	env := &TestEnvironment{
		Name:      name,
		Config:    config,
		Resources: make(map[string]interface{}),
		State: EnvironmentState{
			Status:      EnvironmentStatusReady,
			LastUpdated: time.Now(),
			Resources:   make(map[string]ResourceState),
			Metrics: EnvironmentMetrics{
				ResourceUsage: make(map[string]float64),
			},
		},
	}

	e.environments[name] = env
}

// GetEnvironment gets a test environment
func (e *EnterpriseTestPatterns) GetEnvironment(name string) (*TestEnvironment, error) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	env, exists := e.environments[name]
	if !exists {
		return nil, fmt.Errorf("environment %s not found", name)
	}

	return env, nil
}

// Enterprise test pattern examples

// CreateAPIContractTest creates an API contract test
func (e *EnterpriseTestPatterns) CreateAPIContractTest(apiName, version string, interactions []Interaction) (*Contract, error) {
	contract := &Contract{
		Name:         fmt.Sprintf("%s_contract", apiName),
		Version:      version,
		Provider:     apiName,
		Consumer:     "test_consumer",
		Interactions: interactions,
		Metadata: map[string]interface{}{
			"created_at": time.Now(),
			"test_type":  "api_contract",
		},
	}

	e.contractSuite.AddContract(contract)

	// Create contract test with basic validator
	_, err := e.contractSuite.CreateContractTest(contract.Name, &BasicContractValidator{})
	if err != nil {
		return nil, err
	}

	return contract, nil
}

// CreateChaosScenario creates a chaos engineering scenario
func (e *EnterpriseTestPatterns) CreateChaosScenario(scenarioType string, config map[string]interface{}) error {
	switch scenarioType {
	case "network_latency":
		latency := config["latency"].(time.Duration)
		duration := config["duration"].(time.Duration)
		scenario := NewNetworkLatencyScenario(latency, duration)
		e.chaosTest.AddScenario(scenario)

	case "service_unavailable":
		serviceName := config["service_name"].(string)
		duration := config["duration"].(time.Duration)
		scenario := NewServiceUnavailableScenario(serviceName, duration)
		e.chaosTest.AddScenario(scenario)

	default:
		return fmt.Errorf("unknown chaos scenario type: %s", scenarioType)
	}

	return nil
}

// CreatePerformanceTest creates a performance test
func (e *EnterpriseTestPatterns) CreatePerformanceTest(name string, testFunc func() error) TestCase {
	return TestCase{
		Name:        name,
		Description: fmt.Sprintf("Performance test: %s", name),
		Execute: func(app *EnterpriseTestApp, env *TestEnvironment) error {
			return testFunc()
		},
		Timeout: 30 * time.Second,
		Retries: 3,
	}
}

// CreateMultiEnvironmentTest creates a test that runs across multiple environments
func (e *EnterpriseTestPatterns) CreateMultiEnvironmentTest(name string, testFunc func(env *TestEnvironment) error) error {
	e.mutex.RLock()
	environments := make([]*TestEnvironment, 0, len(e.environments))
	for _, env := range e.environments {
		environments = append(environments, env)
	}
	e.mutex.RUnlock()

	var errors []error
	for _, env := range environments {
		if err := testFunc(env); err != nil {
			errors = append(errors, fmt.Errorf("environment %s: %w", env.Name, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("multi-environment test failures: %v", errors)
	}

	return nil
}

// PatternTestReport generates a comprehensive pattern test report
type PatternTestReport struct {
	SuiteName       string                         `json:"suiteName"`
	StartTime       time.Time                      `json:"startTime"`
	EndTime         time.Time                      `json:"endTime"`
	Duration        time.Duration                  `json:"duration"`
	TotalTests      int                            `json:"totalTests"`
	PassedTests     int                            `json:"passedTests"`
	FailedTests     int                            `json:"failedTests"`
	SkippedTests    int                            `json:"skippedTests"`
	TestResults     []PatternTestResult            `json:"testResults"`
	ContractResults map[string]ContractTestResult  `json:"contractResults,omitempty"`
	ChaosResults    *ChaosMetrics                  `json:"chaosResults,omitempty"`
	PerformanceData map[string]PerformanceBaseline `json:"performanceData,omitempty"`
	Environment     string                         `json:"environment"`
	Metadata        map[string]interface{}         `json:"metadata"`
}

// GenerateReport generates a comprehensive test report
func (e *EnterpriseTestPatterns) GenerateReport(suiteName string, results []TestResult) TestReport {
	// Convert []TestResult to []*TestResult
	testResults := make([]*TestResult, len(results))
	for i := range results {
		testResults[i] = &results[i]
	}

	report := TestReport{
		SuiteName:   suiteName,
		StartTime:   time.Now(),
		TestResults: testResults,
		Metadata:    make(map[string]interface{}),
	}

	// Calculate statistics
	report.TotalTests = len(results)
	for _, result := range results {
		switch result.Status {
		case TestStatusPassed:
			report.PassedTests++
		case TestStatusFailed:
			report.FailedTests++
		case TestStatusSkipped:
			report.SkippedTests++
		}

		if report.StartTime.After(result.StartTime) {
			report.StartTime = result.StartTime
		}
		if report.EndTime.Before(result.EndTime) {
			report.EndTime = result.EndTime
		}
	}

	report.Duration = report.EndTime.Sub(report.StartTime)

	return report
}
