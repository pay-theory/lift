package kernel

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/pay-theory/lift/pkg/observability"
	"github.com/pay-theory/lift/pkg/observability/zap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSTSClient is a mock STS client for testing
type mockSTSClient struct {
	assumeRoleFunc func(ctx context.Context, params *sts.AssumeRoleInput, optFns ...func(*sts.Options)) (*sts.AssumeRoleOutput, error)
}

func (m *mockSTSClient) AssumeRole(ctx context.Context, params *sts.AssumeRoleInput, optFns ...func(*sts.Options)) (*sts.AssumeRoleOutput, error) {
	if m.assumeRoleFunc != nil {
		return m.assumeRoleFunc(ctx, params, optFns...)
	}

	// Default mock response
	expiration := time.Now().Add(1 * time.Hour)
	return &sts.AssumeRoleOutput{
		Credentials: &types.Credentials{
			AccessKeyId:     aws.String("AKIAIOSFODNN7EXAMPLE"),
			SecretAccessKey: aws.String("wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"),
			SessionToken:    aws.String("SESSION_TOKEN"),
			Expiration:      &expiration,
		},
	}, nil
}

// setupTestClient creates a test client with mock dependencies
func setupTestClient(t *testing.T) (*Client, *mockSTSClient) {
	t.Helper()

	// Create logger
	logger, err := zap.NewZapLogger(observability.LoggerConfig{
		Level:  "error", // Set to error to reduce test output
		Format: "json",
	})
	require.NoError(t, err)

	// Logger getter function
	getLogger := func() observability.StructuredLogger {
		return logger
	}

	// Create client with test configuration
	client := &Client{
		accountID:      "123456789012",
		region:         "us-east-1",
		externalID:     "",
		loggerFunc:     getLogger,
		httpClient:     &http.Client{},
		connectTimeout: DefaultConnectTimeout,
		readTimeout:    DefaultReadTimeout,
	}

	// Create mock STS client
	mockSTS := &mockSTSClient{}
	client.stsClient = mockSTS

	return client, mockSTS
}

func TestNewClient(t *testing.T) {
	tests := []struct {
		name        string
		config      ClientConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid configuration",
			config: ClientConfig{
				AccountID: "123456789012",
				Region:    "us-east-1",
			},
			expectError: false,
		},
		{
			name: "missing account ID",
			config: ClientConfig{
				Region: "us-east-1",
			},
			expectError: true,
			errorMsg:    "AccountID is required",
		},
		{
			name: "missing region uses default",
			config: ClientConfig{
				AccountID: "123456789012",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create logger
			logger, err := zap.NewZapLogger(observability.LoggerConfig{
				Level:  "error",
				Format: "json",
			})
			require.NoError(t, err)

			// Logger getter
			getLogger := func() observability.StructuredLogger {
				return logger
			}

			// Create client
			ctx := context.Background()
			client, err := NewClient(ctx, getLogger, tt.config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, client)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				assert.Equal(t, tt.config.AccountID, client.accountID)
				if tt.config.Region != "" {
					assert.Equal(t, tt.config.Region, client.region)
				} else {
					assert.Equal(t, "us-east-1", client.region)
				}
			}
		})
	}
}

func TestBuildFullURL(t *testing.T) {
	client, _ := setupTestClient(t)

	tests := []struct {
		name        string
		baseURL     string
		endpoint    string
		expectedURL string
	}{
		{
			name:        "base URL without trailing slash",
			baseURL:     "https://api.test.com",
			endpoint:    "v1/resource",
			expectedURL: "https://api.test.com/v1/resource",
		},
		{
			name:        "base URL with trailing slash",
			baseURL:     "https://api.test.com/",
			endpoint:    "v1/resource",
			expectedURL: "https://api.test.com/v1/resource",
		},
		{
			name:        "endpoint with query string",
			baseURL:     "https://api.test.com",
			endpoint:    "?param=value",
			expectedURL: "https://api.test.com/?param=value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &CallOptions{
				BaseURL:  tt.baseURL,
				Endpoint: tt.endpoint,
			}
			result := client.buildFullURL(opts)
			assert.Equal(t, tt.expectedURL, result)
		})
	}
}

func TestCall(t *testing.T) {
	client, _ := setupTestClient(t)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "application/json", r.Header.Get("Accept"))

		// Verify SigV4 signature exists
		assert.NotEmpty(t, r.Header.Get("Authorization"))
		assert.Contains(t, r.Header.Get("Authorization"), "AWS4-HMAC-SHA256")

		// Read and verify body
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)

		var requestData map[string]any
		err = json.Unmarshal(body, &requestData)
		assert.NoError(t, err)
		assert.Equal(t, "test_value", requestData["test_key"])

		// Send response
		response := map[string]any{
			"success": true,
			"data":    "test_response",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Override HTTP client for testing
	client.httpClient = server.Client()

	ctx := context.Background()

	opts := &CallOptions{
		BaseURL:  server.URL,
		Endpoint: "test-endpoint",
		Method:   "POST",
		Body: map[string]any{
			"test_key": "test_value",
		},
	}

	resp, err := client.Call(ctx, opts)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestCallWithErrorStatus(t *testing.T) {
	client, _ := setupTestClient(t)

	// Create test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer server.Close()

	client.httpClient = server.Client()

	ctx := context.Background()

	opts := &CallOptions{
		BaseURL:  server.URL,
		Endpoint: "test-endpoint",
		Method:   "POST",
		Body:     map[string]any{},
	}

	resp, err := client.Call(ctx, opts)

	assert.Error(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Contains(t, err.Error(), "kernel service error")
}

func TestResponseUnmarshal(t *testing.T) {
	type TestStruct struct {
		Success bool   `json:"success"`
		Data    string `json:"data"`
	}

	response := &Response{
		StatusCode: 200,
		Body:       []byte(`{"success":true,"data":"test_value"}`),
	}

	var result TestStruct
	err := response.Unmarshal(&result)

	assert.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "test_value", result.Data)
}

func TestKernelClientMiddleware(t *testing.T) {
	client, _ := setupTestClient(t)

	// This test just verifies the middleware function returns
	middleware := KernelClientMiddleware(client)
	assert.NotNil(t, middleware)
}

func TestGetCrossAccountCredentials(t *testing.T) {
	client, mockSTS := setupTestClient(t)

	// Test successful role assumption
	t.Run("successful assume role", func(t *testing.T) {
		ctx := context.Background()

		credentials, err := client.getCrossAccountCredentials(ctx)

		assert.NoError(t, err)
		assert.Equal(t, "AKIAIOSFODNN7EXAMPLE", credentials.AccessKeyID)
		assert.Equal(t, "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", credentials.SecretAccessKey)
		assert.Equal(t, "SESSION_TOKEN", credentials.SessionToken)
	})

	// Test with external ID
	t.Run("assume role with external ID", func(t *testing.T) {
		client.externalID = "test-external-id"
		defer func() { client.externalID = "" }()

		var capturedInput *sts.AssumeRoleInput
		mockSTS.assumeRoleFunc = func(ctx context.Context, params *sts.AssumeRoleInput, optFns ...func(*sts.Options)) (*sts.AssumeRoleOutput, error) {
			capturedInput = params
			expiration := time.Now().Add(1 * time.Hour)
			return &sts.AssumeRoleOutput{
				Credentials: &types.Credentials{
					AccessKeyId:     aws.String("AKIAIOSFODNN7EXAMPLE"),
					SecretAccessKey: aws.String("wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"),
					SessionToken:    aws.String("SESSION_TOKEN"),
					Expiration:      &expiration,
				},
			}, nil
		}

		ctx := context.Background()
		_, err := client.getCrossAccountCredentials(ctx)

		assert.NoError(t, err)
		assert.NotNil(t, capturedInput)
		assert.NotNil(t, capturedInput.ExternalId)
		assert.Equal(t, "test-external-id", *capturedInput.ExternalId)
		assert.True(t, strings.Contains(*capturedInput.RoleArn, "kernel-access-external"))
	})

	// Test role ARN construction
	t.Run("role ARN uses configured account ID", func(t *testing.T) {
		var capturedInput *sts.AssumeRoleInput
		mockSTS.assumeRoleFunc = func(ctx context.Context, params *sts.AssumeRoleInput, optFns ...func(*sts.Options)) (*sts.AssumeRoleOutput, error) {
			capturedInput = params
			expiration := time.Now().Add(1 * time.Hour)
			return &sts.AssumeRoleOutput{
				Credentials: &types.Credentials{
					AccessKeyId:     aws.String("AKIAIOSFODNN7EXAMPLE"),
					SecretAccessKey: aws.String("wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"),
					SessionToken:    aws.String("SESSION_TOKEN"),
					Expiration:      &expiration,
				},
			}, nil
		}

		ctx := context.Background()
		_, err := client.getCrossAccountCredentials(ctx)

		assert.NoError(t, err)
		assert.NotNil(t, capturedInput)
		assert.Contains(t, *capturedInput.RoleArn, client.accountID)
		assert.Contains(t, *capturedInput.RoleArn, "kernel-access")
	})
}

func TestApplyCallDefaults(t *testing.T) {
	client, _ := setupTestClient(t)

	tests := []struct {
		name           string
		opts           *CallOptions
		expectedMethod string
	}{
		{
			name:           "empty method defaults to POST",
			opts:           &CallOptions{},
			expectedMethod: http.MethodPost,
		},
		{
			name:           "explicit method preserved",
			opts:           &CallOptions{Method: http.MethodGet},
			expectedMethod: http.MethodGet,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client.applyCallDefaults(tt.opts)
			assert.Equal(t, tt.expectedMethod, tt.opts.Method)
			assert.Equal(t, client.connectTimeout, tt.opts.ConnectTimeout)
			assert.Equal(t, client.readTimeout, tt.opts.ReadTimeout)
		})
	}
}

func TestMarshalRequestBody(t *testing.T) {
	tests := []struct {
		name        string
		body        any
		expectNil   bool
		expectError bool
	}{
		{
			name:      "nil body",
			body:      nil,
			expectNil: true,
		},
		{
			name:      "map body",
			body:      map[string]any{"key": "value"},
			expectNil: false,
		},
		{
			name:      "struct body",
			body:      struct{ Name string }{Name: "test"},
			expectNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := marshalRequestBody(tt.body)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.expectNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
			}
		})
	}
}
