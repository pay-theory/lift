package security

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSecretsManagerClient struct {
	getSecretValueFunc func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
	updateSecretFunc   func(ctx context.Context, params *secretsmanager.UpdateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.UpdateSecretOutput, error)
	createSecretFunc   func(ctx context.Context, params *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error)
	rotateSecretFunc   func(ctx context.Context, params *secretsmanager.RotateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.RotateSecretOutput, error)
	deleteSecretFunc   func(ctx context.Context, params *secretsmanager.DeleteSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DeleteSecretOutput, error)

	getSecretValueCalls int
	updateSecretCalls   int
	createSecretCalls   int
	rotateSecretCalls   int
	deleteSecretCalls   int
}

func (f *fakeSecretsManagerClient) GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	f.getSecretValueCalls++
	if f.getSecretValueFunc == nil {
		return nil, errors.New("GetSecretValue unexpected call")
	}
	return f.getSecretValueFunc(ctx, params, optFns...)
}

func (f *fakeSecretsManagerClient) UpdateSecret(ctx context.Context, params *secretsmanager.UpdateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.UpdateSecretOutput, error) {
	f.updateSecretCalls++
	if f.updateSecretFunc == nil {
		return nil, errors.New("UpdateSecret unexpected call")
	}
	return f.updateSecretFunc(ctx, params, optFns...)
}

func (f *fakeSecretsManagerClient) CreateSecret(ctx context.Context, params *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
	f.createSecretCalls++
	if f.createSecretFunc == nil {
		return nil, errors.New("CreateSecret unexpected call")
	}
	return f.createSecretFunc(ctx, params, optFns...)
}

func (f *fakeSecretsManagerClient) RotateSecret(ctx context.Context, params *secretsmanager.RotateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.RotateSecretOutput, error) {
	f.rotateSecretCalls++
	if f.rotateSecretFunc == nil {
		return nil, errors.New("RotateSecret unexpected call")
	}
	return f.rotateSecretFunc(ctx, params, optFns...)
}

func (f *fakeSecretsManagerClient) DeleteSecret(ctx context.Context, params *secretsmanager.DeleteSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DeleteSecretOutput, error) {
	f.deleteSecretCalls++
	if f.deleteSecretFunc == nil {
		return nil, errors.New("DeleteSecret unexpected call")
	}
	return f.deleteSecretFunc(ctx, params, optFns...)
}

func TestSecretCache_ManagementMethods(t *testing.T) {
	t.Parallel()

	cache := NewSecretCache(50 * time.Millisecond)
	cache.Set("a", "1")
	cache.Set("b", "2")
	assert.Equal(t, 2, cache.Size())

	cache.Delete("a")
	assert.Equal(t, 1, cache.Size())

	cache.Clear()
	assert.Equal(t, 0, cache.Size())

	cache = NewSecretCache(5 * time.Millisecond)
	cache.Set("expired", "x")
	time.Sleep(10 * time.Millisecond)
	cache.CleanupExpired()
	assert.Equal(t, 0, cache.Size())
}

func TestAWSSecretsManager_GetSecret_PlainCacheHit(t *testing.T) {
	t.Parallel()

	fake := &fakeSecretsManagerClient{
		getSecretValueFunc: func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
			t.Fatalf("expected AWS not to be called")
			return nil, nil
		},
	}

	asm := &AWSSecretsManager{
		client:        fake,
		cache:         NewSecretCache(5 * time.Minute),
		keyPrefix:     "prefix/",
		useEncryption: false,
	}
	asm.cache.Set("token", "cached")

	value, err := asm.GetSecret(context.Background(), "token")
	require.NoError(t, err)
	assert.Equal(t, "cached", value)
	assert.Equal(t, 0, fake.getSecretValueCalls)
}

func TestAWSSecretsManager_GetSecret_FetchesAndCaches(t *testing.T) {
	t.Parallel()

	fake := &fakeSecretsManagerClient{
		getSecretValueFunc: func(_ context.Context, params *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
			require.NotNil(t, params.SecretId)
			assert.Equal(t, "prefix/token", *params.SecretId)
			return &secretsmanager.GetSecretValueOutput{SecretString: aws.String("from-aws")}, nil
		},
	}

	asm := &AWSSecretsManager{
		client:        fake,
		cache:         NewSecretCache(5 * time.Minute),
		keyPrefix:     "prefix/",
		useEncryption: false,
	}

	value, err := asm.GetSecret(context.Background(), "token")
	require.NoError(t, err)
	assert.Equal(t, "from-aws", value)
	assert.Equal(t, 1, fake.getSecretValueCalls)

	value, err = asm.GetSecret(context.Background(), "token")
	require.NoError(t, err)
	assert.Equal(t, "from-aws", value)
	assert.Equal(t, 1, fake.getSecretValueCalls, "second call should hit cache")
}

func TestAWSSecretsManager_GetSecret_EncryptedCacheHitAndDecryptErrorFallback(t *testing.T) {
	t.Parallel()

	cache, err := NewEncryptedSecretCache(5*time.Minute, []byte("key"))
	require.NoError(t, err)

	// Cache hit should short-circuit AWS.
	require.NoError(t, cache.Set("token", "cached"))

	fake := &fakeSecretsManagerClient{
		getSecretValueFunc: func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
			t.Fatalf("expected AWS not to be called on cache hit")
			return nil, nil
		},
	}

	asm := &AWSSecretsManager{
		client:         fake,
		encryptedCache: cache,
		keyPrefix:      "prefix/",
		useEncryption:  true,
	}

	value, err := asm.GetSecret(context.Background(), "token")
	require.NoError(t, err)
	assert.Equal(t, "cached", value)
	assert.Equal(t, 0, fake.getSecretValueCalls)

	// Corrupt the cached value to force a decrypt error and verify fallback.
	cache.secrets["token"].EncryptedValue = []byte("corrupt")
	fake.getSecretValueFunc = func(_ context.Context, params *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
		require.NotNil(t, params.SecretId)
		assert.Equal(t, "prefix/token", *params.SecretId)
		return &secretsmanager.GetSecretValueOutput{SecretString: aws.String("from-aws")}, nil
	}

	value, err = asm.GetSecret(context.Background(), "token")
	require.NoError(t, err)
	assert.Equal(t, "from-aws", value)
	assert.Equal(t, 1, fake.getSecretValueCalls)
}

func TestAWSSecretsManager_GetSecret_Errors(t *testing.T) {
	t.Parallel()

	t.Run("aws error does not include secret name", func(t *testing.T) {
		t.Parallel()

		fake := &fakeSecretsManagerClient{
			getSecretValueFunc: func(context.Context, *secretsmanager.GetSecretValueInput, ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
				return nil, errors.New("aws down")
			},
		}

		asm := &AWSSecretsManager{client: fake, cache: NewSecretCache(5 * time.Minute), keyPrefix: "prefix/"}
		_, err := asm.GetSecret(context.Background(), "token")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to retrieve secret")
		assert.False(t, strings.Contains(err.Error(), "token"))
	})

	t.Run("nil secret string", func(t *testing.T) {
		t.Parallel()

		fake := &fakeSecretsManagerClient{
			getSecretValueFunc: func(context.Context, *secretsmanager.GetSecretValueInput, ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
				return &secretsmanager.GetSecretValueOutput{}, nil
			},
		}

		asm := &AWSSecretsManager{client: fake, cache: NewSecretCache(5 * time.Minute)}
		_, err := asm.GetSecret(context.Background(), "token")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not in expected format")
	})
}

func TestAWSSecretsManager_PutRotateDeleteAndJSON(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("put updates existing secret", func(t *testing.T) {
		t.Parallel()

		fake := &fakeSecretsManagerClient{
			updateSecretFunc: func(_ context.Context, params *secretsmanager.UpdateSecretInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.UpdateSecretOutput, error) {
				require.NotNil(t, params.SecretId)
				require.NotNil(t, params.SecretString)
				assert.Equal(t, "prefix/token", *params.SecretId)
				assert.Equal(t, "value", *params.SecretString)
				return &secretsmanager.UpdateSecretOutput{}, nil
			},
		}

		asm := &AWSSecretsManager{
			client:        fake,
			cache:         NewSecretCache(5 * time.Minute),
			keyPrefix:     "prefix/",
			useEncryption: false,
		}

		require.NoError(t, asm.PutSecret(ctx, "token", "value"))
		assert.Equal(t, 1, fake.updateSecretCalls)
		assert.Equal(t, "value", asm.cache.Get("token"))
	})

	t.Run("put creates secret when missing", func(t *testing.T) {
		t.Parallel()

		fake := &fakeSecretsManagerClient{
			updateSecretFunc: func(context.Context, *secretsmanager.UpdateSecretInput, ...func(*secretsmanager.Options)) (*secretsmanager.UpdateSecretOutput, error) {
				return nil, &types.ResourceNotFoundException{}
			},
			createSecretFunc: func(_ context.Context, params *secretsmanager.CreateSecretInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
				require.NotNil(t, params.Name)
				require.NotNil(t, params.SecretString)
				require.NotNil(t, params.Description)
				assert.Equal(t, "prefix/token", *params.Name)
				assert.Equal(t, "value", *params.SecretString)
				assert.Contains(t, *params.Description, "Lift framework secret")
				return &secretsmanager.CreateSecretOutput{}, nil
			},
		}

		asm := &AWSSecretsManager{
			client:        fake,
			cache:         NewSecretCache(5 * time.Minute),
			keyPrefix:     "prefix/",
			useEncryption: false,
		}

		require.NoError(t, asm.PutSecret(ctx, "token", "value"))
		assert.Equal(t, 1, fake.updateSecretCalls)
		assert.Equal(t, 1, fake.createSecretCalls)
	})

	t.Run("put update failure", func(t *testing.T) {
		t.Parallel()

		fake := &fakeSecretsManagerClient{
			updateSecretFunc: func(context.Context, *secretsmanager.UpdateSecretInput, ...func(*secretsmanager.Options)) (*secretsmanager.UpdateSecretOutput, error) {
				return nil, errors.New("boom")
			},
		}

		asm := &AWSSecretsManager{client: fake, cache: NewSecretCache(5 * time.Minute), keyPrefix: "prefix/"}
		err := asm.PutSecret(ctx, "token", "value")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update secret")
	})

	t.Run("put create failure", func(t *testing.T) {
		t.Parallel()

		fake := &fakeSecretsManagerClient{
			updateSecretFunc: func(context.Context, *secretsmanager.UpdateSecretInput, ...func(*secretsmanager.Options)) (*secretsmanager.UpdateSecretOutput, error) {
				return nil, &types.ResourceNotFoundException{}
			},
			createSecretFunc: func(context.Context, *secretsmanager.CreateSecretInput, ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
				return nil, errors.New("create failed")
			},
		}

		asm := &AWSSecretsManager{client: fake, cache: NewSecretCache(5 * time.Minute), keyPrefix: "prefix/"}
		err := asm.PutSecret(ctx, "token", "value")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create secret")
	})

	t.Run("rotate invalidates cache", func(t *testing.T) {
		t.Parallel()

		fake := &fakeSecretsManagerClient{
			rotateSecretFunc: func(_ context.Context, params *secretsmanager.RotateSecretInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.RotateSecretOutput, error) {
				require.NotNil(t, params.SecretId)
				assert.Equal(t, "prefix/token", *params.SecretId)
				return &secretsmanager.RotateSecretOutput{}, nil
			},
		}

		asm := &AWSSecretsManager{client: fake, cache: NewSecretCache(5 * time.Minute), keyPrefix: "prefix/"}
		asm.cache.Set("token", "cached")
		require.NoError(t, asm.RotateSecret(ctx, "token"))
		assert.Equal(t, "", asm.cache.Get("token"))
	})

	t.Run("delete removes cache", func(t *testing.T) {
		t.Parallel()

		fake := &fakeSecretsManagerClient{
			deleteSecretFunc: func(_ context.Context, params *secretsmanager.DeleteSecretInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.DeleteSecretOutput, error) {
				require.NotNil(t, params.SecretId)
				assert.Equal(t, "prefix/token", *params.SecretId)
				return &secretsmanager.DeleteSecretOutput{}, nil
			},
		}

		asm := &AWSSecretsManager{client: fake, cache: NewSecretCache(5 * time.Minute), keyPrefix: "prefix/"}
		asm.cache.Set("token", "cached")
		require.NoError(t, asm.DeleteSecret(ctx, "token"))
		assert.Equal(t, "", asm.cache.Get("token"))
	})

	t.Run("json helpers", func(t *testing.T) {
		t.Parallel()

		type payload struct {
			A string `json:"a"`
			B int    `json:"b"`
		}

		fake := &fakeSecretsManagerClient{
			updateSecretFunc: func(_ context.Context, params *secretsmanager.UpdateSecretInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.UpdateSecretOutput, error) {
				require.NotNil(t, params.SecretString)
				assert.JSONEq(t, `{"a":"x","b":2}`, *params.SecretString)
				return &secretsmanager.UpdateSecretOutput{}, nil
			},
			getSecretValueFunc: func(context.Context, *secretsmanager.GetSecretValueInput, ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
				return &secretsmanager.GetSecretValueOutput{SecretString: aws.String(`{"a":"x","b":2}`)}, nil
			},
		}

		asm := &AWSSecretsManager{client: fake, cache: NewSecretCache(5 * time.Minute)}
		require.NoError(t, asm.PutJSONSecret(ctx, "token", payload{A: "x", B: 2}))

		var out payload
		require.NoError(t, asm.GetJSONSecret(ctx, "token", &out))
		assert.Equal(t, payload{A: "x", B: 2}, out)

		fake.getSecretValueFunc = func(context.Context, *secretsmanager.GetSecretValueInput, ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
			return &secretsmanager.GetSecretValueOutput{SecretString: aws.String("{bad")}, nil
		}
		asm.cache.Delete("token")
		require.Error(t, asm.GetJSONSecret(ctx, "token", &out))
	})
}

func TestFileSecretsProvider_AdditionalUtilities(t *testing.T) {
	t.Parallel()

	provider := NewFileSecretsProvider(t.TempDir())
	ctx := context.Background()

	_, err := provider.GetSecret(ctx, "missing")
	require.Error(t, err)

	err = provider.RotateSecret(ctx, "missing")
	require.Error(t, err)
	history := provider.GetRotationHistory("missing")
	require.Len(t, history, 1)
	assert.False(t, history[0].Success)
	assert.Equal(t, "secret not found", history[0].Error)

	require.NoError(t, provider.PutSecret(ctx, "jwt", "abc.def.ghi"))
	require.NoError(t, provider.RotateSecret(ctx, "jwt"))
	rotated, err := provider.GetSecret(ctx, "jwt")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(rotated, "abc.def.ghi-rotated-"))

	require.NoError(t, provider.PutSecret(ctx, "short", "short"))
	require.NoError(t, provider.RotateSecret(ctx, "short"))
	rotated, err = provider.GetSecret(ctx, "short")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(rotated, "short-rotated-"))

	assert.True(t, provider.IsRotationEnabled())
	provider.SetRotationEnabled(false)
	assert.False(t, provider.IsRotationEnabled())
	provider.SetRotationEnabled(true)
	assert.True(t, provider.IsRotationEnabled())

	provider.ClearRotationHistory()
	assert.Empty(t, provider.GetAllRotationHistory())

	require.NoError(t, provider.PutSecret(ctx, "delete", "value"))
	require.NoError(t, provider.RotateSecret(ctx, "delete"))
	require.NoError(t, provider.DeleteSecret(ctx, "delete"))
	_, err = provider.GetSecret(ctx, "delete")
	require.Error(t, err)
	assert.Empty(t, provider.GetRotationHistory("delete"))
}

func TestMockSecretsProvider(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	msp := NewMockSecretsProvider()

	_, err := msp.GetSecret(ctx, "missing")
	require.Error(t, err)

	require.NoError(t, msp.PutSecret(ctx, "k", "v"))
	val, err := msp.GetSecret(ctx, "k")
	require.NoError(t, err)
	assert.Equal(t, "v", val)

	require.NoError(t, msp.RotateSecret(ctx, "k"))
	val, err = msp.GetSecret(ctx, "k")
	require.NoError(t, err)
	assert.Equal(t, "v-rotated", val)

	require.NoError(t, msp.DeleteSecret(ctx, "k"))
	_, err = msp.GetSecret(ctx, "k")
	require.Error(t, err)

	msp.SetSecret("k2", "v2")
	val, err = msp.GetSecret(ctx, "k2")
	require.NoError(t, err)
	assert.Equal(t, "v2", val)
}
