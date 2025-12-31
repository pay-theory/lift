package security

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDataProtectionManager_ValidateDataAccessFromGDPR(t *testing.T) {
	t.Parallel()

	manager, err := NewDataProtectionManager(DataProtectionConfig{
		EncryptionKey: "test-key",
		RegionRestrictions: map[DataClassification][]string{
			DataRestricted: {"us-east-1"},
		},
		AccessControls: map[DataClassification][]string{
			DataRestricted: {"role"},
		},
	})
	require.NoError(t, err)

	for _, tc := range []struct {
		name          string
		purpose       string
		expectClass   DataClassification
		expectAllowed bool
	}{
		{name: "restricted", purpose: "restricted", expectClass: DataRestricted, expectAllowed: true},
		{name: "processing", purpose: "processing", expectClass: DataRestricted, expectAllowed: true},
		{name: "confidential", purpose: "confidential", expectClass: DataConfidential, expectAllowed: true},
		{name: "internal", purpose: "internal", expectClass: DataInternal, expectAllowed: true},
		{name: "default public", purpose: "something", expectClass: DataPublic, expectAllowed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := manager.ValidateDataAccessFromGDPR(DataAccessRequest{
				UserID:      "user",
				RequestType: "access",
				Purpose:     tc.purpose,
				Region:      "us-east-1",
				Scope:       []string{"field"},
				Metadata:    map[string]any{"k": "v"},
			})
			require.NotNil(t, result)
			assert.Equal(t, tc.expectAllowed, result.Allowed)

			// Ensure the request conversion path is exercised.
			if tc.expectClass == DataRestricted {
				assert.Empty(t, result.Violations)
			}
		})
	}

	// Already a DataProtectionRequest.
	res := manager.ValidateDataAccessFromGDPR(DataProtectionRequest{UserID: "user", Classification: DataPublic, Region: "us-east-1"})
	assert.True(t, res.Allowed)

	// Unsupported type.
	res = manager.ValidateDataAccessFromGDPR(123)
	assert.False(t, res.Allowed)
	assert.Contains(t, res.Violations, "Unsupported request type")
}

func TestDataProtectionManager_ProtectData_InternalBranchesAndErrors(t *testing.T) {
	t.Parallel()

	manager, err := NewDataProtectionManager(DataProtectionConfig{
		DefaultClassification: DataInternal,
		EncryptionKey:         "test-key",
		MaskingRules: map[string]MaskingRule{
			"secret": {Type: "tokenize", Replacement: "*"},
		},
	})
	require.NoError(t, err)

	internalCtx := &DataContext{
		Data:           map[string]any{"secret": "value"},
		Classification: DataInternal,
		Fields:         map[string]DataClassification{"secret": DataRestricted},
	}

	res, err := manager.ProtectData(internalCtx, DataProtectionRequest{UserID: "u", Region: "us-east-1", Purpose: "external"})
	require.NoError(t, err)
	assert.NotNil(t, res.MaskedData)

	res, err = manager.ProtectData(internalCtx, DataProtectionRequest{UserID: "u", Region: "us-east-1", Purpose: "internal"})
	require.NoError(t, err)
	assert.Equal(t, internalCtx.Data, res.Data)

	// Masking error (json marshal fails).
	bad := &DataContext{
		Data:           make(chan int),
		Classification: DataRestricted,
		Fields:         map[string]DataClassification{"x": DataRestricted},
	}
	_, err = manager.ProtectData(bad, DataProtectionRequest{UserID: "u", Region: "us-east-1", Purpose: "display", Classification: DataRestricted})
	require.Error(t, err)

	// Encryption error (json marshal fails).
	_, err = manager.ProtectData(bad, DataProtectionRequest{UserID: "u", Region: "us-east-1", Purpose: "processing", Classification: DataRestricted})
	require.Error(t, err)
}

func TestDataProtectionManager_MaskingDefaultsAndRules(t *testing.T) {
	t.Parallel()

	manager, err := NewDataProtectionManager(DataProtectionConfig{
		DefaultClassification: DataInternal,
		EncryptionKey:         "test-key",
	})
	require.NoError(t, err)

	// applyMaskingRule non-string should be returned unchanged.
	v, err := manager.applyMaskingRule(123, MaskingRule{Type: "full", Replacement: "*"})
	require.NoError(t, err)
	assert.Equal(t, 123, v)

	// tokenize rule.
	token, err := manager.applyMaskingRule("secret", MaskingRule{Type: "tokenize", Replacement: "*"})
	require.NoError(t, err)
	assert.IsType(t, "", token)
	assert.NotEqual(t, "secret", token)

	// partial with short input.
	masked, err := manager.applyMaskingRule("abc", MaskingRule{Type: "partial", Replacement: "*"})
	require.NoError(t, err)
	assert.Equal(t, "***", masked)

	// unknown rule should be no-op.
	v, err = manager.applyMaskingRule("v", MaskingRule{Type: "unknown", Replacement: "*"})
	require.NoError(t, err)
	assert.Equal(t, "v", v)

	assert.Equal(t, "***", manager.applyDefaultMasking("abc", DataRestricted))
	assert.Equal(t, "****", manager.applyDefaultMasking("1234", DataConfidential))
	assert.Equal(t, "12**56", manager.applyDefaultMasking("123456", DataConfidential))
	assert.Equal(t, "v", manager.applyDefaultMasking("v", DataInternal))
	assert.Equal(t, "v", manager.applyDefaultMasking("v", DataPublic))
	assert.Equal(t, "v", manager.applyDefaultMasking("v", "other"))
}

func TestAESEncryptor_ErrorCases(t *testing.T) {
	t.Parallel()

	encryptor, err := NewAESEncryptor("test-key")
	require.NoError(t, err)

	_, err = encryptor.Encrypt(make(chan int))
	require.Error(t, err)

	var out map[string]any
	require.Error(t, encryptor.Decrypt("not-base64", &out))

	// Too-short ciphertext (less than nonce size).
	short := base64.StdEncoding.EncodeToString([]byte{1, 2, 3})
	require.Error(t, encryptor.Decrypt(short, &out))
}

func TestDataProtectionMiddleware_ConfigErrorAndGetManagerError(t *testing.T) {
	t.Parallel()

	mw := DataProtection(DataProtectionConfig{})
	handler := LiftHandlerFunc(func(LiftContext) error { return nil })
	ctx := newTestLiftContext("u", "t", &recordingLogger{})

	err := mw(handler).Handle(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "data protection service unavailable")

	// Missing manager in context.
	_, err = GetDataProtectionManager(ctx)
	require.Error(t, err)

	// Wrong type.
	ctx.Set("data_protection", "nope")
	_, err = GetDataProtectionManager(ctx)
	require.Error(t, err)
}

func TestDataProtectionManager_ClassifyFieldHelpers(t *testing.T) {
	t.Parallel()

	manager, err := NewDataProtectionManager(DataProtectionConfig{
		DefaultClassification: DataInternal,
		EncryptionKey:         "test-key",
	})
	require.NoError(t, err)

	assert.Equal(t, DataPublic, manager.classifyField("client_ip", "1.2.3.4"))
	assert.True(t, manager.isIPField("device_ip"))
	assert.True(t, manager.isIPField("ip_device"))
	assert.True(t, manager.isIPField("foo_ip_bar"))
	assert.True(t, manager.isIPField("forwarded_ip_header"))
	assert.True(t, manager.isIPField("forwardedip"))

	assert.Equal(t, DataPublic, manager.classifyField("customer_id", "id"))

	assert.Equal(t, DataInternal, manager.classifyField("service_key", "v"))
	assert.Equal(t, DataRestricted, manager.classifyField("secret_key", "v"))
	assert.Equal(t, DataRestricted, manager.classifyField("my_private_key", "v"))

	assert.Equal(t, DataConfidential, manager.classifyField("password_hash", "v"))
	assert.Equal(t, DataRestricted, manager.classifyField("routing_number", "v"))
	assert.Equal(t, DataInternal, manager.classifyField("email_address", "v"))

	assert.Equal(t, DataRestricted, manager.classifyField("passport_number", "v"))
	assert.Equal(t, DataConfidential, manager.classifyField("salary_amount", "v"))
	assert.Equal(t, DataInternal, manager.classifyField("request_body", "v"))
	assert.Equal(t, DataInternal, manager.classifyField("birth_date", "v"))

	// Value-based restricted detection when no field pattern matches.
	assert.Equal(t, DataRestricted, manager.classifyField("misc", "4111-1111-1111-1111"))
}

func TestDataProtectionManager_ClassifyData_JSONMarshalFailure(t *testing.T) {
	t.Parallel()

	manager, err := NewDataProtectionManager(DataProtectionConfig{
		DefaultClassification: DataInternal,
		EncryptionKey:         "test-key",
	})
	require.NoError(t, err)

	ctx := manager.ClassifyData(make(chan int), map[string]any{"user_id": "u"})
	assert.Equal(t, DataInternal, ctx.Classification)
	assert.Equal(t, "u", ctx.UserID)
}

func TestMaskData_UnmarshalError(t *testing.T) {
	t.Parallel()

	manager, err := NewDataProtectionManager(DataProtectionConfig{
		DefaultClassification: DataInternal,
		EncryptionKey:         "test-key",
	})
	require.NoError(t, err)

	// json.Marshal will succeed, but Unmarshal into map will fail if it's a JSON array.
	_, err = manager.maskData([]string{"a"}, map[string]DataClassification{})
	require.Error(t, err)
}

func TestDataAccessResult_ExpiresAtSet(t *testing.T) {
	t.Parallel()

	manager, err := NewDataProtectionManager(DataProtectionConfig{
		EncryptionKey: "test-key",
		RetentionPolicies: map[DataClassification]time.Duration{
			DataInternal: time.Hour,
		},
	})
	require.NoError(t, err)

	res := manager.ValidateDataAccess(DataProtectionRequest{UserID: "u", Classification: DataInternal})
	assert.False(t, res.ExpiresAt.IsZero())
}
