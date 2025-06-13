package security

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

// AWSSecretsManager implements the SecretsProvider interface using AWS Secrets Manager
type AWSSecretsManager struct {
	client    *secretsmanager.Client
	cache     *SecretCache
	keyPrefix string
	region    string
}

// SecretCache provides in-memory caching for secrets with TTL
type SecretCache struct {
	secrets map[string]*CachedSecret
	mu      sync.RWMutex
	ttl     time.Duration
}

// CachedSecret represents a cached secret with expiration
type CachedSecret struct {
	Value     string
	ExpiresAt time.Time
}

// NewAWSSecretsManager creates a new AWS Secrets Manager provider
func NewAWSSecretsManager(ctx context.Context, region, keyPrefix string) (*AWSSecretsManager, error) {
	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := secretsmanager.NewFromConfig(cfg)

	return &AWSSecretsManager{
		client:    client,
		cache:     NewSecretCache(5 * time.Minute), // 5-minute cache TTL
		keyPrefix: keyPrefix,
		region:    region,
	}, nil
}

// NewSecretCache creates a new secret cache with the specified TTL
func NewSecretCache(ttl time.Duration) *SecretCache {
	return &SecretCache{
		secrets: make(map[string]*CachedSecret),
		ttl:     ttl,
	}
}

// GetSecret retrieves a secret from AWS Secrets Manager (with caching)
func (asm *AWSSecretsManager) GetSecret(ctx context.Context, name string) (string, error) {
	// Check cache first
	if value := asm.cache.Get(name); value != "" {
		return value, nil
	}

	// Build full secret name with prefix
	fullName := asm.buildSecretName(name)

	// Retrieve from AWS Secrets Manager
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(fullName),
	}

	result, err := asm.client.GetSecretValue(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to get secret %s: %w", fullName, err)
	}

	if result.SecretString == nil {
		return "", fmt.Errorf("secret %s has no string value", fullName)
	}

	value := *result.SecretString

	// Cache the secret
	asm.cache.Set(name, value)

	return value, nil
}

// PutSecret stores a secret in AWS Secrets Manager
func (asm *AWSSecretsManager) PutSecret(ctx context.Context, name string, value string) error {
	fullName := asm.buildSecretName(name)

	// Try to update existing secret first
	updateInput := &secretsmanager.UpdateSecretInput{
		SecretId:     aws.String(fullName),
		SecretString: aws.String(value),
	}

	_, err := asm.client.UpdateSecret(ctx, updateInput)
	if err != nil {
		// If secret doesn't exist, create it
		var notFound *types.ResourceNotFoundException
		if errors.As(err, &notFound) {
			createInput := &secretsmanager.CreateSecretInput{
				Name:         aws.String(fullName),
				SecretString: aws.String(value),
				Description:  aws.String(fmt.Sprintf("Lift framework secret: %s", name)),
			}

			_, createErr := asm.client.CreateSecret(ctx, createInput)
			if createErr != nil {
				return fmt.Errorf("failed to create secret %s: %w", fullName, createErr)
			}
		} else {
			return fmt.Errorf("failed to update secret %s: %w", fullName, err)
		}
	}

	// Update cache
	asm.cache.Set(name, value)

	return nil
}

// RotateSecret initiates rotation for a secret
func (asm *AWSSecretsManager) RotateSecret(ctx context.Context, name string) error {
	fullName := asm.buildSecretName(name)

	input := &secretsmanager.RotateSecretInput{
		SecretId: aws.String(fullName),
	}

	_, err := asm.client.RotateSecret(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to rotate secret %s: %w", fullName, err)
	}

	// Invalidate cache
	asm.cache.Delete(name)

	return nil
}

// DeleteSecret removes a secret from AWS Secrets Manager
func (asm *AWSSecretsManager) DeleteSecret(ctx context.Context, name string) error {
	fullName := asm.buildSecretName(name)

	input := &secretsmanager.DeleteSecretInput{
		SecretId:                   aws.String(fullName),
		ForceDeleteWithoutRecovery: aws.Bool(false), // Allow recovery period
	}

	_, err := asm.client.DeleteSecret(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete secret %s: %w", fullName, err)
	}

	// Remove from cache
	asm.cache.Delete(name)

	return nil
}

// GetJSONSecret retrieves and unmarshals a JSON secret
func (asm *AWSSecretsManager) GetJSONSecret(ctx context.Context, name string, target interface{}) error {
	value, err := asm.GetSecret(ctx, name)
	if err != nil {
		return err
	}

	if err := json.Unmarshal([]byte(value), target); err != nil {
		return fmt.Errorf("failed to unmarshal JSON secret %s: %w", name, err)
	}

	return nil
}

// PutJSONSecret marshals and stores a JSON secret
func (asm *AWSSecretsManager) PutJSONSecret(ctx context.Context, name string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON secret %s: %w", name, err)
	}

	return asm.PutSecret(ctx, name, string(data))
}

// buildSecretName constructs the full secret name with prefix
func (asm *AWSSecretsManager) buildSecretName(name string) string {
	if asm.keyPrefix == "" {
		return name
	}
	return asm.keyPrefix + name
}

// Cache methods

// Get retrieves a value from the cache
func (c *SecretCache) Get(key string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	secret, exists := c.secrets[key]
	if !exists {
		return ""
	}

	// Check if expired
	if time.Now().After(secret.ExpiresAt) {
		delete(c.secrets, key)
		return ""
	}

	return secret.Value
}

// Set stores a value in the cache with TTL
func (c *SecretCache) Set(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.secrets[key] = &CachedSecret{
		Value:     value,
		ExpiresAt: time.Now().Add(c.ttl),
	}
}

// Delete removes a value from the cache
func (c *SecretCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.secrets, key)
}

// Clear removes all values from the cache
func (c *SecretCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.secrets = make(map[string]*CachedSecret)
}

// Size returns the number of cached secrets
func (c *SecretCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.secrets)
}

// CleanupExpired removes expired secrets from the cache
func (c *SecretCache) CleanupExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, secret := range c.secrets {
		if now.After(secret.ExpiresAt) {
			delete(c.secrets, key)
		}
	}
}

// FileSecretsProvider implements SecretsProvider for local file-based secrets (development only)
type FileSecretsProvider struct {
	basePath string
	cache    map[string]string
	mu       sync.RWMutex
}

// NewFileSecretsProvider creates a file-based secrets provider for development
func NewFileSecretsProvider(basePath string) *FileSecretsProvider {
	return &FileSecretsProvider{
		basePath: basePath,
		cache:    make(map[string]string),
	}
}

// GetSecret retrieves a secret from a file
func (fsp *FileSecretsProvider) GetSecret(ctx context.Context, name string) (string, error) {
	// This is a simple implementation for development use
	// In production, always use AWS Secrets Manager
	fsp.mu.RLock()
	value, exists := fsp.cache[name]
	fsp.mu.RUnlock()

	if exists {
		return value, nil
	}

	return "", fmt.Errorf("secret %s not found in file provider", name)
}

// PutSecret stores a secret in memory (file provider)
func (fsp *FileSecretsProvider) PutSecret(ctx context.Context, name string, value string) error {
	fsp.mu.Lock()
	defer fsp.mu.Unlock()

	fsp.cache[name] = value
	return nil
}

// RotateSecret is not implemented for file provider
func (fsp *FileSecretsProvider) RotateSecret(ctx context.Context, name string) error {
	return fmt.Errorf("rotation not supported by file provider")
}

// DeleteSecret removes a secret from memory
func (fsp *FileSecretsProvider) DeleteSecret(ctx context.Context, name string) error {
	fsp.mu.Lock()
	defer fsp.mu.Unlock()

	delete(fsp.cache, name)
	return nil
}

// MockSecretsProvider implements SecretsProvider for testing
type MockSecretsProvider struct {
	secrets map[string]string
	mu      sync.RWMutex
}

// NewMockSecretsProvider creates a mock secrets provider for testing
func NewMockSecretsProvider() *MockSecretsProvider {
	return &MockSecretsProvider{
		secrets: make(map[string]string),
	}
}

// GetSecret retrieves a mock secret
func (msp *MockSecretsProvider) GetSecret(ctx context.Context, name string) (string, error) {
	msp.mu.RLock()
	defer msp.mu.RUnlock()

	value, exists := msp.secrets[name]
	if !exists {
		return "", fmt.Errorf("secret %s not found", name)
	}
	return value, nil
}

// PutSecret stores a mock secret
func (msp *MockSecretsProvider) PutSecret(ctx context.Context, name string, value string) error {
	msp.mu.Lock()
	defer msp.mu.Unlock()

	msp.secrets[name] = value
	return nil
}

// RotateSecret simulates secret rotation
func (msp *MockSecretsProvider) RotateSecret(ctx context.Context, name string) error {
	msp.mu.Lock()
	defer msp.mu.Unlock()

	if _, exists := msp.secrets[name]; !exists {
		return fmt.Errorf("secret %s not found", name)
	}

	// Simulate rotation by appending "-rotated"
	msp.secrets[name] = msp.secrets[name] + "-rotated"
	return nil
}

// DeleteSecret removes a mock secret
func (msp *MockSecretsProvider) DeleteSecret(ctx context.Context, name string) error {
	msp.mu.Lock()
	defer msp.mu.Unlock()

	delete(msp.secrets, name)
	return nil
}

// SetSecret is a convenience method for testing
func (msp *MockSecretsProvider) SetSecret(name, value string) {
	msp.mu.Lock()
	defer msp.mu.Unlock()

	msp.secrets[name] = value
}
