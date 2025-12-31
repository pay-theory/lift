package security

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSSMClient struct {
	getParameterCalls int
	getParameterFunc  func(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
}

func (f *fakeSSMClient) GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	f.getParameterCalls++
	if f.getParameterFunc == nil {
		return nil, errors.New("unexpected call")
	}
	return f.getParameterFunc(ctx, params, optFns...)
}

func TestSSMIPAuthorizer_IsAuthorizedIP_CacheAndValidation(t *testing.T) {
	t.Parallel()

	client := &fakeSSMClient{}
	authorizer := &SSMIPAuthorizer{
		ssmClient: client,
		cache:     cache.New(time.Minute, time.Minute),
		cacheTTL:  time.Minute,
	}

	_, err := authorizer.IsAuthorizedIP(context.Background(), "1.2.3.4", "")
	require.Error(t, err)

	param := "param-name"
	cacheKey := "ssm:ip-list:" + param

	// Cache hit should avoid SSM call.
	authorizer.cache.Set(cacheKey, []string{"1.2.3.4"}, cache.DefaultExpiration)
	ok, err := authorizer.IsAuthorizedIP(context.Background(), "1.2.3.4", param)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, 0, client.getParameterCalls)

	// Wrong cached type should be ignored and trigger fetch.
	authorizer.cache.Set(cacheKey, "wrong-type", cache.DefaultExpiration)
	client.getParameterFunc = func(_ context.Context, params *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
		require.NotNil(t, params.Name)
		assert.Equal(t, param, *params.Name)
		return &ssm.GetParameterOutput{Parameter: &types.Parameter{Value: aws.String("1.2.3.4,5.6.7.8")}}, nil
	}

	ok, err = authorizer.IsAuthorizedIP(context.Background(), "5.6.7.8", param)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, 1, client.getParameterCalls)

	// Now it should be cached as []string and avoid another call.
	ok, err = authorizer.IsAuthorizedIP(context.Background(), "5.6.7.8", param)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, 1, client.getParameterCalls)
}

func TestSSMIPAuthorizer_IsAuthorizedIP_ErrorCases(t *testing.T) {
	t.Parallel()

	param := "param-name"

	t.Run("ssm error is sanitized", func(t *testing.T) {
		t.Parallel()

		client := &fakeSSMClient{
			getParameterFunc: func(context.Context, *ssm.GetParameterInput, ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
				return nil, errors.New("ssm down")
			},
		}
		authorizer := &SSMIPAuthorizer{ssmClient: client, cache: cache.New(time.Minute, time.Minute), cacheTTL: time.Minute}

		_, err := authorizer.IsAuthorizedIP(context.Background(), "1.2.3.4", param)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to retrieve IP authorization configuration")
		assert.False(t, strings.Contains(err.Error(), param))
	})

	t.Run("invalid parameter output", func(t *testing.T) {
		t.Parallel()

		client := &fakeSSMClient{
			getParameterFunc: func(context.Context, *ssm.GetParameterInput, ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
				return &ssm.GetParameterOutput{}, nil
			},
		}
		authorizer := &SSMIPAuthorizer{ssmClient: client, cache: cache.New(time.Minute, time.Minute), cacheTTL: time.Minute}

		_, err := authorizer.IsAuthorizedIP(context.Background(), "1.2.3.4", param)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "IP authorization configuration is invalid")

		client.getParameterFunc = func(context.Context, *ssm.GetParameterInput, ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
			return &ssm.GetParameterOutput{Parameter: &types.Parameter{}}, nil
		}
		_, err = authorizer.IsAuthorizedIP(context.Background(), "1.2.3.4", param)
		require.Error(t, err)
	})
}

func TestIPAuthorizationService_ValidatesSourceIP(t *testing.T) {
	t.Parallel()

	client := &fakeSSMClient{
		getParameterFunc: func(context.Context, *ssm.GetParameterInput, ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
			return &ssm.GetParameterOutput{Parameter: &types.Parameter{Value: aws.String("1.2.3.4")}}, nil
		},
	}
	authorizer := &SSMIPAuthorizer{ssmClient: client, cache: cache.New(time.Minute, time.Minute), cacheTTL: time.Minute}
	service := &IPAuthorizationService{authorizer: authorizer, ssmParameterName: "param"}

	_, err := service.IsAuthorizedIP(context.Background(), "")
	require.Error(t, err)

	ok, err := service.IsAuthorizedIP(context.Background(), "1.2.3.4")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestCheckIPAuthorization_InputValidation(t *testing.T) {
	t.Parallel()

	_, err := CheckIPAuthorization(context.Background(), "", nil, "param")
	require.Error(t, err)

	_, err = CheckIPAuthorization(context.Background(), "1.2.3.4", nil, "")
	require.Error(t, err)
}

func TestNewIPAuthorizationServiceFromEnv_ValidationErrors(t *testing.T) {
	t.Parallel()

	_, err := NewIPAuthorizationServiceFromEnv(context.Background(), "")
	require.Error(t, err)
}

func TestNewSSMIPAuthorizer_LoadsConfig(t *testing.T) {
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

	authorizer, err := NewSSMIPAuthorizer(context.Background())
	require.NoError(t, err)
	require.NotNil(t, authorizer)
}

func TestNewIPAuthorizationServiceFromEnv_BuildsParameterName(t *testing.T) {
	t.Setenv("PARTNER", "paytheory")
	t.Setenv("STAGE", "prod")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

	service, err := NewIPAuthorizationServiceFromEnv(context.Background(), "component")
	require.NoError(t, err)
	require.NotNil(t, service)
	assert.Equal(t, "pt-partner-paytheory-prod-component", service.ssmParameterName)
}

func TestNewIPAuthorizationServiceFromEnv_MissingEnvVars(t *testing.T) {
	t.Setenv("PARTNER", "")
	t.Setenv("STAGE", "")

	_, err := NewIPAuthorizationServiceFromEnv(context.Background(), "component")
	require.Error(t, err)
}
