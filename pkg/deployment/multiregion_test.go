package deployment

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type recordingDeployer struct {
	mu sync.Mutex

	initErr    error
	deployErr  error
	destroyErr error
	outputs    map[string]any

	initializeCount int
	deployCount     int
	destroyCount    int
	lastInitConfig  *StackDeploymentConfig
}

func (d *recordingDeployer) Initialize(_ context.Context, cfg *StackDeploymentConfig) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.initializeCount++
	d.lastInitConfig = cfg
	return d.initErr
}

func (d *recordingDeployer) Deploy(context.Context) (*DeploymentResult, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.deployCount++
	if d.deployErr != nil {
		return nil, d.deployErr
	}

	outputs := d.outputs
	if outputs == nil {
		outputs = map[string]any{}
	}

	stackName := ""
	if d.lastInitConfig != nil {
		stackName = d.lastInitConfig.StackName
	}

	return &DeploymentResult{
		Outputs:   outputs,
		StackName: stackName,
		Success:   true,
	}, nil
}

func (d *recordingDeployer) Destroy(context.Context) (*DeploymentResult, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.destroyCount++
	if d.destroyErr != nil {
		return nil, d.destroyErr
	}

	return &DeploymentResult{
		Outputs:   map[string]any{},
		StackName: "",
		Success:   true,
	}, nil
}

func TestMultiRegionDeployer_NewMultiRegionDeployerInitializesState(t *testing.T) {
	config := MultiRegionConfig{
		PrimaryRegion: "us-east-1",
		Regions:       []string{"us-east-1", "us-west-2"},
	}

	infraConfig := InfrastructureConfig{
		ApplicationName: "test-app",
		Environment:     "dev",
		Tags: map[string]string{
			"Environment": "dev",
		},
	}

	deployer := NewMultiRegionDeployer(config, infraConfig)
	require.NotNil(t, deployer.dnsManager)
	require.NotNil(t, deployer.loadBalancer)

	for _, region := range config.Regions {
		status, ok := deployer.deploymentStatus[region]
		require.True(t, ok)
		require.Equal(t, StatusPending, status.Status)
		require.Equal(t, HealthUnknown, status.Health)
		require.NotNil(t, status.Endpoints)
		require.NotNil(t, deployer.deployers[region])
		require.NotNil(t, deployer.healthCheckers[region])
	}
}

func TestMultiRegionDeployer_DeployAllRollingSuccess(t *testing.T) {
	regions := []string{"us-east-1", "us-west-2"}
	config := MultiRegionConfig{
		PrimaryRegion: regions[0],
		Regions:       regions,
	}

	deployer := NewMultiRegionDeployer(config, InfrastructureConfig{
		ApplicationName: "test-app",
		Environment:     "dev",
	})

	fakes := map[string]*recordingDeployer{
		regions[0]: {outputs: map[string]any{"ServiceEndpoint": "https://east.example.com"}},
		regions[1]: {outputs: map[string]any{"ServiceEndpoint": "https://west.example.com"}},
	}
	for region, fake := range fakes {
		deployer.deployers[region] = fake
	}

	err := deployer.DeployAll(context.Background(), DeploymentStrategyConfig{
		Type:              "rolling",
		BatchSize:         0,
		BatchDelay:        time.Nanosecond,
		RollbackOnFailure: true,
	})
	require.NoError(t, err)

	for _, region := range regions {
		status := deployer.deploymentStatus[region]
		require.Equal(t, StatusDeployed, status.Status)
		require.NotEmpty(t, status.Endpoints)

		fake := fakes[region]
		fake.mu.Lock()
		require.Equal(t, 1, fake.initializeCount)
		require.Equal(t, 1, fake.deployCount)
		require.Equal(t, 0, fake.destroyCount)
		require.NotNil(t, fake.lastInitConfig)
		fake.mu.Unlock()
	}
}

func TestMultiRegionDeployer_DeployAllRollingFailureRollsBackBatch(t *testing.T) {
	regions := []string{"us-east-1"}
	config := MultiRegionConfig{
		PrimaryRegion: regions[0],
		Regions:       regions,
	}

	deployer := NewMultiRegionDeployer(config, InfrastructureConfig{
		ApplicationName: "test-app",
		Environment:     "dev",
	})

	fake := &recordingDeployer{
		deployErr: errors.New("deploy failed"),
	}
	deployer.deployers[regions[0]] = fake

	err := deployer.DeployAll(context.Background(), DeploymentStrategyConfig{
		Type:              "rolling",
		BatchSize:         1,
		RollbackOnFailure: true,
	})
	require.Error(t, err)

	fake.mu.Lock()
	require.Equal(t, 1, fake.initializeCount)
	require.Equal(t, 1, fake.deployCount)
	require.Equal(t, 1, fake.destroyCount)
	fake.mu.Unlock()

	status := deployer.deploymentStatus[regions[0]]
	require.Equal(t, StatusRolledBack, status.Status)
}

func TestMultiRegionDeployer_DeployAllCanarySuccess(t *testing.T) {
	regions := []string{"us-east-1", "us-west-2"}
	config := MultiRegionConfig{
		PrimaryRegion: regions[0],
		Regions:       regions,
	}

	deployer := NewMultiRegionDeployer(config, InfrastructureConfig{
		ApplicationName: "test-app",
		Environment:     "dev",
	})

	fakes := map[string]*recordingDeployer{
		regions[0]: {outputs: map[string]any{"ServiceEndpoint": "https://east.example.com"}},
		regions[1]: {outputs: map[string]any{"ServiceEndpoint": "https://west.example.com"}},
	}
	for region, fake := range fakes {
		deployer.deployers[region] = fake
	}

	err := deployer.DeployAll(context.Background(), DeploymentStrategyConfig{
		Type:             "canary",
		CanaryPercentage: 25,
		CanaryDuration:   0,
	})
	require.NoError(t, err)

	for _, region := range regions {
		fake := fakes[region]
		fake.mu.Lock()
		require.Equal(t, 1, fake.initializeCount)
		require.Equal(t, 1, fake.deployCount)
		fake.mu.Unlock()
	}
}

func TestMultiRegionDeployer_DeployAllCanaryContextCancellationTriggersRollback(t *testing.T) {
	regions := []string{"us-east-1", "us-west-2"}
	config := MultiRegionConfig{
		PrimaryRegion: regions[0],
		Regions:       regions,
	}

	deployer := NewMultiRegionDeployer(config, InfrastructureConfig{
		ApplicationName: "test-app",
		Environment:     "dev",
	})

	fakes := map[string]*recordingDeployer{
		regions[0]: {outputs: map[string]any{"ServiceEndpoint": "https://east.example.com"}},
		regions[1]: {outputs: map[string]any{"ServiceEndpoint": "https://west.example.com"}},
	}
	for region, fake := range fakes {
		deployer.deployers[region] = fake
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := deployer.DeployAll(ctx, DeploymentStrategyConfig{
		Type:           "canary",
		CanaryDuration: time.Hour,
	})
	require.ErrorIs(t, err, context.Canceled)

	fakes[regions[0]].mu.Lock()
	require.Equal(t, 1, fakes[regions[0]].deployCount)
	fakes[regions[0]].mu.Unlock()

	fakes[regions[1]].mu.Lock()
	require.Equal(t, 0, fakes[regions[1]].deployCount)
	fakes[regions[1]].mu.Unlock()
}

func TestMultiRegionDeployer_HealthMonitoringUpdatesHealthyRegions(t *testing.T) {
	regions := []string{"us-east-1", "us-west-2"}
	config := MultiRegionConfig{
		PrimaryRegion: regions[0],
		Regions:       regions,
	}

	deployer := NewMultiRegionDeployer(config, InfrastructureConfig{
		ApplicationName: "test-app",
		Environment:     "dev",
	})

	for _, region := range regions {
		status := deployer.deploymentStatus[region]
		status.Endpoints = map[string]string{"https://example.com": "ok"}
		deployer.deploymentStatus[region] = status
	}

	deployer.performHealthChecks(context.Background())
	require.ElementsMatch(t, regions, deployer.GetHealthyRegions())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		deployer.StartHealthMonitoring(ctx, time.Millisecond)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("health monitoring did not stop")
	}
}

func TestMultiRegionDeployer_GetDeploymentStatusReturnsCopy(t *testing.T) {
	config := MultiRegionConfig{
		PrimaryRegion: "us-east-1",
		Regions:       []string{"us-east-1"},
	}
	deployer := NewMultiRegionDeployer(config, InfrastructureConfig{
		ApplicationName: "test-app",
		Environment:     "dev",
	})

	status := deployer.GetDeploymentStatus()
	require.Contains(t, status, "us-east-1")

	delete(status, "us-east-1")
	require.Contains(t, deployer.deploymentStatus, "us-east-1")
}

func TestMultiRegionDeployer_DeployAllDefaultsToParallel(t *testing.T) {
	regions := []string{"us-east-1", "us-west-2"}
	config := MultiRegionConfig{
		PrimaryRegion: regions[0],
		Regions:       regions,
	}

	deployer := NewMultiRegionDeployer(config, InfrastructureConfig{
		ApplicationName: "test-app",
		Environment:     "dev",
	})

	fakes := map[string]*recordingDeployer{
		regions[0]: {outputs: map[string]any{"ServiceEndpoint": "https://east.example.com"}},
		regions[1]: {outputs: map[string]any{"ServiceEndpoint": "https://west.example.com"}},
	}
	for region, fake := range fakes {
		deployer.deployers[region] = fake
	}

	err := deployer.DeployAll(context.Background(), DeploymentStrategyConfig{Type: "unknown"})
	require.NoError(t, err)

	for _, region := range regions {
		fake := fakes[region]
		fake.mu.Lock()
		require.Equal(t, 1, fake.deployCount)
		fake.mu.Unlock()
	}
}

func TestMultiRegionDeployer_HealthCheckBatchSetsHealthy(t *testing.T) {
	regions := []string{"us-east-1", "us-west-2"}
	config := MultiRegionConfig{
		PrimaryRegion: regions[0],
		Regions:       regions,
	}

	deployer := NewMultiRegionDeployer(config, InfrastructureConfig{
		ApplicationName: "test-app",
		Environment:     "dev",
	})

	for _, region := range regions {
		status := deployer.deploymentStatus[region]
		status.Endpoints = map[string]string{"https://example.com": "ok"}
		deployer.deploymentStatus[region] = status
	}

	err := deployer.healthCheckBatch(context.Background(), regions, 0)
	require.NoError(t, err)
	require.ElementsMatch(t, regions, deployer.GetHealthyRegions())

	delete(deployer.healthCheckers, regions[1])
	err = deployer.healthCheckBatch(context.Background(), regions, 0)
	require.Error(t, err)
}

func TestMultiRegionDeployer_DeployAllBlueGreenSuccess(t *testing.T) {
	regions := []string{"us-east-1", "us-west-2"}
	greenRegions := []string{"us-east-1-green", "us-west-2-green"}
	config := MultiRegionConfig{
		PrimaryRegion: regions[0],
		Regions:       regions,
	}

	deployer := NewMultiRegionDeployer(config, InfrastructureConfig{
		ApplicationName: "test-app",
		Environment:     "dev",
	})

	greenDeployers := map[string]*recordingDeployer{
		greenRegions[0]: {outputs: map[string]any{"ServiceEndpoint": "https://east-green.example.com"}},
		greenRegions[1]: {outputs: map[string]any{"ServiceEndpoint": "https://west-green.example.com"}},
	}

	for _, region := range greenRegions {
		deployer.deployers[region] = greenDeployers[region]
		deployer.healthCheckers[region] = NewRegionHealthChecker(region, HealthCheckConfig{})
		deployer.deploymentStatus[region] = RegionDeploymentStatus{
			Region:    region,
			Status:    StatusPending,
			Health:    HealthUnknown,
			Endpoints: make(map[string]string),
		}
	}

	err := deployer.DeployAll(context.Background(), DeploymentStrategyConfig{
		Type:                   "blue-green",
		HealthCheckGracePeriod: 0,
	})
	require.NoError(t, err)

	for _, region := range greenRegions {
		status := deployer.deploymentStatus[region]
		require.Equal(t, StatusDeployed, status.Status)
		require.Equal(t, HealthHealthy, status.Health)
		require.NotEmpty(t, status.Endpoints)
	}
}

func TestMultiRegionDeployer_DeployBatchAndRollbackBatchErrorBranches(t *testing.T) {
	config := MultiRegionConfig{
		PrimaryRegion: "us-east-1",
		Regions:       []string{"us-east-1"},
	}
	deployer := NewMultiRegionDeployer(config, InfrastructureConfig{
		ApplicationName: "test-app",
		Environment:     "dev",
	})

	delete(deployer.deployers, "us-east-1")
	require.Error(t, deployer.deployBatch(context.Background(), []string{"us-east-1"}))

	initErrDeployer := &recordingDeployer{initErr: errors.New("init failed")}
	deployer.deployers["us-east-1"] = initErrDeployer
	require.Error(t, deployer.deployBatch(context.Background(), []string{"us-east-1"}))

	delete(deployer.deployers, "us-east-1")
	require.Error(t, deployer.rollbackBatch(context.Background(), []string{"us-east-1"}))

	destroyErrDeployer := &recordingDeployer{destroyErr: errors.New("destroy failed")}
	deployer.deployers["us-east-1"] = destroyErrDeployer
	require.Error(t, deployer.rollbackBatch(context.Background(), []string{"us-east-1"}))
}

func TestMultiRegionDeployer_StartHealthMonitoringTicks(t *testing.T) {
	config := MultiRegionConfig{
		PrimaryRegion: "us-east-1",
		Regions:       []string{"us-east-1"},
	}
	deployer := NewMultiRegionDeployer(config, InfrastructureConfig{
		ApplicationName: "test-app",
		Environment:     "dev",
	})

	delete(deployer.healthCheckers, "us-east-1")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		deployer.StartHealthMonitoring(ctx, time.Millisecond)
		close(done)
	}()

	time.Sleep(3 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("health monitoring did not stop")
	}
}
