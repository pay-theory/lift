package security

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

// AWSSecretsManager implements the SecretsProvider interface using AWS Secrets Manager
type AWSSecretsManager struct {
	client         *secretsmanager.Client
	cache          *SecretCache          // Legacy plain text cache (deprecated)
	encryptedCache *EncryptedSecretCache // New encrypted cache
	keyPrefix      string
	region         string
	useEncryption  bool // Flag to control cache type
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

// NewAWSSecretsManager creates a new AWS Secrets Manager provider with plain text cache (deprecated)
func NewAWSSecretsManager(ctx context.Context, region, keyPrefix string) (*AWSSecretsManager, error) {
	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := secretsmanager.NewFromConfig(cfg)

	return &AWSSecretsManager{
		client:        client,
		cache:         NewSecretCache(5 * time.Minute), // 5-minute cache TTL
		keyPrefix:     keyPrefix,
		region:        region,
		useEncryption: false, // Legacy mode
	}, nil
}

// NewSecureAWSSecretsManager creates a new AWS Secrets Manager provider with encrypted cache
func NewSecureAWSSecretsManager(ctx context.Context, region, keyPrefix string, encryptionKey []byte) (*AWSSecretsManager, error) {
	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := secretsmanager.NewFromConfig(cfg)

	// Create encrypted cache
	encryptedCache, err := NewEncryptedSecretCache(5*time.Minute, encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create encrypted cache: %w", err)
	}

	return &AWSSecretsManager{
		client:         client,
		encryptedCache: encryptedCache,
		keyPrefix:      keyPrefix,
		region:         region,
		useEncryption:  true,
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
	// Check cache first (encrypted or plain text)
	if asm.useEncryption && asm.encryptedCache != nil {
		if value, err := asm.encryptedCache.Get(name); err != nil {
			// Log error but continue to fetch from AWS
		} else if value != "" {
			return value, nil
		}
	} else if asm.cache != nil {
		if value := asm.cache.Get(name); value != "" {
			return value, nil
		}
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

	// Cache the secret (encrypted or plain text)
	if asm.useEncryption && asm.encryptedCache != nil {
		if err := asm.encryptedCache.Set(name, value); err != nil {
			// Log error but don't fail the request
			fmt.Printf("Warning: failed to cache secret in encrypted cache: %v\n", err)
		}
	} else if asm.cache != nil {
		asm.cache.Set(name, value)
	}

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

	// Update cache (encrypted or plain text)
	if asm.useEncryption && asm.encryptedCache != nil {
		if err := asm.encryptedCache.Set(name, value); err != nil {
			fmt.Printf("Warning: failed to update secret in encrypted cache: %v\n", err)
		}
	} else if asm.cache != nil {
		asm.cache.Set(name, value)
	}

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

	// Invalidate cache (encrypted or plain text)
	if asm.useEncryption && asm.encryptedCache != nil {
		asm.encryptedCache.Delete(name)
	} else if asm.cache != nil {
		asm.cache.Delete(name)
	}

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

	// Remove from cache (encrypted or plain text)
	if asm.useEncryption && asm.encryptedCache != nil {
		asm.encryptedCache.Delete(name)
	} else if asm.cache != nil {
		asm.cache.Delete(name)
	}

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
	// Rotation simulation
	rotationHistory map[string][]RotationRecord
	enableRotation  bool
}

// RotationRecord tracks rotation events for testing
type RotationRecord struct {
	Timestamp  time.Time `json:"timestamp"`
	OldValue   string    `json:"old_value,omitempty"` // For testing only
	NewValue   string    `json:"new_value,omitempty"` // For testing only
	RotationID string    `json:"rotation_id"`
	Method     string    `json:"method"`
	Success    bool      `json:"success"`
	Error      string    `json:"error,omitempty"`
}

// NewFileSecretsProvider creates a file-based secrets provider for development
func NewFileSecretsProvider(basePath string) *FileSecretsProvider {
	return &FileSecretsProvider{
		basePath:        basePath,
		cache:           make(map[string]string),
		rotationHistory: make(map[string][]RotationRecord),
		enableRotation:  true,
	}
}

// NewFileSecretsProviderWithConfig creates a file-based secrets provider with configuration
func NewFileSecretsProviderWithConfig(basePath string, enableRotation bool) *FileSecretsProvider {
	return &FileSecretsProvider{
		basePath:        basePath,
		cache:           make(map[string]string),
		rotationHistory: make(map[string][]RotationRecord),
		enableRotation:  enableRotation,
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

// RotateSecret implements rotation for file provider with simulation
func (fsp *FileSecretsProvider) RotateSecret(ctx context.Context, name string) error {
	if !fsp.enableRotation {
		return fmt.Errorf("rotation not enabled for file provider")
	}

	fsp.mu.Lock()
	defer fsp.mu.Unlock()

	// Check if secret exists
	oldValue, exists := fsp.cache[name]
	if !exists {
		record := RotationRecord{
			Timestamp:  time.Now(),
			RotationID: fmt.Sprintf("rot_%d", time.Now().UnixNano()),
			Method:     "file_provider_simulation",
			Success:    false,
			Error:      "secret not found",
		}
		fsp.addRotationRecord(name, record)
		return fmt.Errorf("secret %s not found", name)
	}

	// Simulate rotation by generating a new value
	newValue := fsp.generateRotatedValue(oldValue, name)

	// Update the secret with new value
	fsp.cache[name] = newValue

	// Record the rotation
	record := RotationRecord{
		Timestamp:  time.Now(),
		OldValue:   oldValue, // Only for testing
		NewValue:   newValue, // Only for testing
		RotationID: fmt.Sprintf("rot_%d", time.Now().UnixNano()),
		Method:     "file_provider_simulation",
		Success:    true,
	}
	fsp.addRotationRecord(name, record)

	return nil
}

// generateRotatedValue generates a new value for rotation simulation
func (fsp *FileSecretsProvider) generateRotatedValue(oldValue, secretName string) string {
	// For development, we'll use different strategies based on the secret type

	// If it looks like a JWT token or API key, append a version
	if strings.Contains(oldValue, ".") || len(oldValue) > 32 {
		timestamp := time.Now().Unix()
		return fmt.Sprintf("%s-rotated-%d", oldValue, timestamp)
	}

	// If it looks like a password, generate a new one
	if len(oldValue) >= 8 && len(oldValue) <= 64 {
		return fsp.generateSimulatedPassword()
	}

	// For other types, append rotation indicator
	return fmt.Sprintf("%s-rotated-%d", oldValue, time.Now().Unix())
}

// generateSimulatedPassword generates a simulated password for testing
func (fsp *FileSecretsProvider) generateSimulatedPassword() string {
	// Generate a simple password for testing
	timestamp := time.Now().Unix()
	return fmt.Sprintf("TestPass%d!", timestamp)
}

// addRotationRecord adds a rotation record to history
func (fsp *FileSecretsProvider) addRotationRecord(name string, record RotationRecord) {
	if fsp.rotationHistory[name] == nil {
		fsp.rotationHistory[name] = make([]RotationRecord, 0)
	}

	// Keep only last 10 rotation records per secret
	history := fsp.rotationHistory[name]
	if len(history) >= 10 {
		history = history[1:] // Remove oldest
	}

	fsp.rotationHistory[name] = append(history, record)
}

// GetRotationHistory returns rotation history for a secret (testing/debugging)
func (fsp *FileSecretsProvider) GetRotationHistory(name string) []RotationRecord {
	fsp.mu.RLock()
	defer fsp.mu.RUnlock()

	history := fsp.rotationHistory[name]
	if history == nil {
		return []RotationRecord{}
	}

	// Return a copy to prevent external modification
	result := make([]RotationRecord, len(history))
	copy(result, history)
	return result
}

// GetAllRotationHistory returns rotation history for all secrets (testing/debugging)
func (fsp *FileSecretsProvider) GetAllRotationHistory() map[string][]RotationRecord {
	fsp.mu.RLock()
	defer fsp.mu.RUnlock()

	result := make(map[string][]RotationRecord)
	for name, history := range fsp.rotationHistory {
		historyCopy := make([]RotationRecord, len(history))
		copy(historyCopy, history)
		result[name] = historyCopy
	}

	return result
}

// SimulateRotationFailure simulates a rotation failure for testing
func (fsp *FileSecretsProvider) SimulateRotationFailure(ctx context.Context, name string, errorMessage string) error {
	if !fsp.enableRotation {
		return fmt.Errorf("rotation not enabled for file provider")
	}

	fsp.mu.Lock()
	defer fsp.mu.Unlock()

	record := RotationRecord{
		Timestamp:  time.Now(),
		RotationID: fmt.Sprintf("rot_fail_%d", time.Now().UnixNano()),
		Method:     "file_provider_simulation_failure",
		Success:    false,
		Error:      errorMessage,
	}
	fsp.addRotationRecord(name, record)

	return fmt.Errorf("simulated rotation failure: %s", errorMessage)
}

// SetRotationEnabled enables or disables rotation for testing
func (fsp *FileSecretsProvider) SetRotationEnabled(enabled bool) {
	fsp.mu.Lock()
	defer fsp.mu.Unlock()
	fsp.enableRotation = enabled
}

// IsRotationEnabled returns whether rotation is enabled
func (fsp *FileSecretsProvider) IsRotationEnabled() bool {
	fsp.mu.RLock()
	defer fsp.mu.RUnlock()
	return fsp.enableRotation
}

// ClearRotationHistory clears all rotation history (testing utility)
func (fsp *FileSecretsProvider) ClearRotationHistory() {
	fsp.mu.Lock()
	defer fsp.mu.Unlock()
	fsp.rotationHistory = make(map[string][]RotationRecord)
}

// DeleteSecret removes a secret from memory
func (fsp *FileSecretsProvider) DeleteSecret(ctx context.Context, name string) error {
	fsp.mu.Lock()
	defer fsp.mu.Unlock()

	delete(fsp.cache, name)

	// Also clear rotation history for the deleted secret
	delete(fsp.rotationHistory, name)

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
