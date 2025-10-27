package deployment

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCDKDeployer_InitializeAndDeploy_DefaultSynthesizer(t *testing.T) {
	config := InfrastructureConfig{
		ApplicationName: "test-app",
		Metadata: map[string]any{
			"default_endpoint": "https://api.test-app.dev",
		},
	}

	deployer := NewCDKDeployer("test-app", "test-app-us-east-1", "us-east-1", config)

	stackCfg := &StackDeploymentConfig{
		ProjectName: "test-app",
		StackName:   "test-app-us-east-1",
		Region:      "us-east-1",
		Config: map[string]string{
			"environment": "dev",
		},
		Tags: map[string]string{
			"Environment": "dev",
		},
		Context: map[string]string{
			"feature": "blue",
		},
	}

	require.NoError(t, deployer.Initialize(context.Background(), stackCfg))

	// Mutate original config to confirm defensive copy
	stackCfg.Config["environment"] = "prod"
	stackCfg.Tags["Environment"] = "prod"
	stackCfg.Context["feature"] = "green"

	result, err := deployer.Deploy(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, "test-app-us-east-1", result.StackName)
	assert.Equal(t, "https://api.test-app.dev", result.Outputs["ServiceEndpoint"])
	assert.NotZero(t, result.Duration)
}

func TestCDKDeployer_Deploy_UsesCustomSynthesizer(t *testing.T) {
	expectedErr := errors.New("synthetic failure")

	synth := func(context.Context, *StackDeploymentConfig, InfrastructureConfig) (map[string]any, []ResourceSummary, error) {
		return nil, nil, expectedErr
	}

	deployer := NewCDKDeployer(
		"app",
		"app-eu",
		"eu-west-1",
		InfrastructureConfig{},
		WithCDKSynthesizer(synth),
	)

	cfg := &StackDeploymentConfig{
		ProjectName: "app",
		StackName:   "app-eu",
		Region:      "eu-west-1",
	}

	require.NoError(t, deployer.Initialize(context.Background(), cfg))

	result, err := deployer.Deploy(context.Background())
	require.Error(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, expectedErr.Error())
}

func TestCDKDeployer_DestroyIsNoop(t *testing.T) {
	deployer := NewCDKDeployer("app", "app-stack", "us-west-2", InfrastructureConfig{})
	require.NoError(t, deployer.Initialize(context.Background(), &StackDeploymentConfig{
		ProjectName: "app",
		StackName:   "app-stack",
		Region:      "us-west-2",
	}))

	result, err := deployer.Destroy(context.Background())
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "app-stack", result.StackName)
	assert.LessOrEqual(t, result.Duration, time.Duration(0))
}
