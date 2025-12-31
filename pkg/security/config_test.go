package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultSecurityConfig_ValidateAfterKeyPathsSet(t *testing.T) {
	t.Parallel()

	cfg := DefaultSecurityConfig()
	require.NotNil(t, cfg)

	cfg.JWTConfig.PublicKeyPath = "/tmp/public.pem"
	cfg.JWTConfig.PrivateKeyPath = "/tmp/private.pem"

	require.NoError(t, cfg.Validate())
	assert.Equal(t, "RS256", cfg.JWTConfig.SigningMethod)
	assert.True(t, cfg.RBACEnabled)
	assert.True(t, cfg.TenantValidation)
	assert.True(t, cfg.EncryptionAtRest)
	assert.Greater(t, cfg.MaxRequestSize, int64(0))
}

func TestSecurityConfigValidate_Errors(t *testing.T) {
	t.Parallel()

	t.Run("missing signing method", func(t *testing.T) {
		t.Parallel()

		cfg := &SecurityConfig{
			JWTConfig:      JWTConfig{SigningMethod: ""},
			APIKeyConfig:   APIKeyConfig{MinLength: 16},
			MaxRequestSize: 1,
		}

		err := cfg.Validate()
		require.Error(t, err)
		var secErr *SecurityError
		require.ErrorAs(t, err, &secErr)
		assert.Equal(t, "INVALID_CONFIG", secErr.Code)
	})

	t.Run("hs256 missing secret key", func(t *testing.T) {
		t.Parallel()

		cfg := &SecurityConfig{
			JWTConfig:      JWTConfig{SigningMethod: "HS256"},
			APIKeyConfig:   APIKeyConfig{MinLength: 16},
			MaxRequestSize: 1,
		}

		err := cfg.Validate()
		require.Error(t, err)
		var secErr *SecurityError
		require.ErrorAs(t, err, &secErr)
		assert.Equal(t, "INVALID_CONFIG", secErr.Code)
	})

	t.Run("rs256 missing key paths", func(t *testing.T) {
		t.Parallel()

		cfg := &SecurityConfig{
			JWTConfig:      JWTConfig{SigningMethod: "RS256", PublicKeyPath: "/tmp/public.pem"},
			APIKeyConfig:   APIKeyConfig{MinLength: 16},
			MaxRequestSize: 1,
		}

		err := cfg.Validate()
		require.Error(t, err)
		var secErr *SecurityError
		require.ErrorAs(t, err, &secErr)
		assert.Equal(t, "INVALID_CONFIG", secErr.Code)
	})

	t.Run("invalid max request size", func(t *testing.T) {
		t.Parallel()

		cfg := &SecurityConfig{
			JWTConfig:      JWTConfig{SigningMethod: "HS256", SecretKey: "secret"},
			APIKeyConfig:   APIKeyConfig{MinLength: 16},
			MaxRequestSize: 0,
		}

		err := cfg.Validate()
		require.Error(t, err)
		var secErr *SecurityError
		require.ErrorAs(t, err, &secErr)
		assert.Equal(t, "INVALID_CONFIG", secErr.Code)
	})

	t.Run("api key min length too small", func(t *testing.T) {
		t.Parallel()

		cfg := &SecurityConfig{
			JWTConfig:      JWTConfig{SigningMethod: "HS256", SecretKey: "secret"},
			APIKeyConfig:   APIKeyConfig{MinLength: 15},
			MaxRequestSize: 1,
		}

		err := cfg.Validate()
		require.Error(t, err)
		var secErr *SecurityError
		require.ErrorAs(t, err, &secErr)
		assert.Equal(t, "INVALID_CONFIG", secErr.Code)
	})
}

func TestSecurityError(t *testing.T) {
	t.Parallel()

	err := NewSecurityError("CODE", "message")
	require.NotNil(t, err)
	assert.Equal(t, "CODE", err.Code)
	assert.Equal(t, "message", err.Message)
	assert.Equal(t, "message", err.Error())
}

