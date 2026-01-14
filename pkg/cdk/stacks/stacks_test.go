package stacks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/stretchr/testify/require"
)

func assetDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bootstrap"), []byte("x"), 0o644))
	return dir
}

func TestNewMicroserviceStack(t *testing.T) {
	app := awscdk.NewApp(nil)
	code := assetDir(t)

	stack := NewMicroserviceStack(app, "Microservice", &MicroserviceStackProps{
		ServiceName:    "svc",
		CodePath:       code,
		MemorySize:     512,
		EnableDatabase: true,
		Environment: map[string]string{
			"STAGE": "test",
		},
	})
	require.NotNil(t, stack)

	stackNoDB := NewMicroserviceStack(app, "MicroserviceNoDB", &MicroserviceStackProps{
		ServiceName:    "svc2",
		CodePath:       code,
		MemorySize:     256,
		EnableDatabase: false,
	})
	require.NotNil(t, stackNoDB)
}

func TestNewEventDrivenStack(t *testing.T) {
	app := awscdk.NewApp(nil)
	apiCode := assetDir(t)
	processorCode := assetDir(t)

	stack := NewEventDrivenStack(app, "EventDriven", &EventDrivenStackProps{
		AppName:                "app",
		ApiCodePath:            apiCode,
		EventProcessorCodePath: processorCode,
		EventBusName:           "custom-bus",
		EnableDLQ:              true,
	})
	require.NotNil(t, stack)

	stackDefaultBus := NewEventDrivenStack(app, "EventDrivenDefaultBus", &EventDrivenStackProps{
		AppName:                "app2",
		ApiCodePath:            apiCode,
		EventProcessorCodePath: processorCode,
		EventBusName:           "default",
		EnableDLQ:              false,
	})
	require.NotNil(t, stackDefaultBus)
}

func TestNewMultiTenantSaaSStack(t *testing.T) {
	app := awscdk.NewApp(nil)
	code := assetDir(t)

	stack := NewMultiTenantSaaSStack(app, "SaaS", &MultiTenantSaaSStackProps{
		AppName:           "saas",
		CodePath:          code,
		DomainName:        "example.com",
		CertificateArn:    "arn:aws:acm:us-east-1:123456789012:certificate/abc",
		EnableFileStorage: true,
		EnableAuth:        true,
	})
	require.NotNil(t, stack)

	stackNoExtras := NewMultiTenantSaaSStack(app, "SaaSNoExtras", &MultiTenantSaaSStackProps{
		AppName:           "saas2",
		CodePath:          code,
		EnableFileStorage: false,
		EnableAuth:        false,
	})
	require.NotNil(t, stackNoExtras)
}
