package cli

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLookPath creates a LookPathFunc that simulates found/missing binaries.
func mockLookPath(found map[string]bool) LookPathFunc {
	return func(name string) (string, error) {
		if found[name] {
			return "/usr/bin/" + name, nil
		}
		return "", errors.New("executable file not found in $PATH")
	}
}

func TestCheckGo_Found(t *testing.T) {
	lookup := mockLookPath(map[string]bool{"go": true})
	err := CheckGo(lookup)
	require.NoError(t, err)
}

func TestCheckGo_NotFound(t *testing.T) {
	lookup := mockLookPath(map[string]bool{})
	err := CheckGo(lookup)

	require.Error(t, err)
	assert.True(t, IsPrereqError(err))

	var prereqErr *PrereqError
	require.True(t, errors.As(err, &prereqErr))
	assert.Equal(t, "go", prereqErr.Binary)
	assert.Contains(t, prereqErr.Message, "go not found")
	assert.Contains(t, prereqErr.Message, "https://go.dev/doc/install")
}

func TestCheckCDK_Found(t *testing.T) {
	lookup := mockLookPath(map[string]bool{"cdk": true})
	err := CheckCDK(lookup)
	require.NoError(t, err)
}

func TestCheckCDK_NotFound(t *testing.T) {
	lookup := mockLookPath(map[string]bool{})
	err := CheckCDK(lookup)

	require.Error(t, err)
	assert.True(t, IsPrereqError(err))

	var prereqErr *PrereqError
	require.True(t, errors.As(err, &prereqErr))
	assert.Equal(t, "cdk", prereqErr.Binary)
	assert.Contains(t, prereqErr.Message, "cdk not found")
	assert.Contains(t, prereqErr.Message, "npm install -g aws-cdk")
}

func TestCheckNode_Found(t *testing.T) {
	lookup := mockLookPath(map[string]bool{"node": true})
	err := CheckNode(lookup)
	require.NoError(t, err)
}

func TestCheckNode_NotFound(t *testing.T) {
	lookup := mockLookPath(map[string]bool{})
	err := CheckNode(lookup)

	require.Error(t, err)
	assert.True(t, IsPrereqError(err))

	var prereqErr *PrereqError
	require.True(t, errors.As(err, &prereqErr))
	assert.Equal(t, "node", prereqErr.Binary)
	assert.Contains(t, prereqErr.Message, "node not found")
	assert.Contains(t, prereqErr.Message, "https://nodejs.org/")
}

func TestCheckPrereqs_AllFound(t *testing.T) {
	lookup := mockLookPath(map[string]bool{"go": true, "cdk": true, "node": true})
	err := CheckPrereqs(lookup, "go", "cdk", "node")
	require.NoError(t, err)
}

func TestCheckPrereqs_GoMissing(t *testing.T) {
	lookup := mockLookPath(map[string]bool{"cdk": true, "node": true})
	err := CheckPrereqs(lookup, "go", "cdk")

	require.Error(t, err)
	var prereqErr *PrereqError
	require.True(t, errors.As(err, &prereqErr))
	assert.Equal(t, "go", prereqErr.Binary)
}

func TestCheckPrereqs_CDKMissing(t *testing.T) {
	lookup := mockLookPath(map[string]bool{"go": true, "node": true})
	err := CheckPrereqs(lookup, "go", "cdk")

	require.Error(t, err)
	var prereqErr *PrereqError
	require.True(t, errors.As(err, &prereqErr))
	assert.Equal(t, "cdk", prereqErr.Binary)
}

func TestCheckPrereqs_MultipleMissing_ReturnsFirst(t *testing.T) {
	lookup := mockLookPath(map[string]bool{})
	err := CheckPrereqs(lookup, "go", "cdk", "node")

	require.Error(t, err)
	var prereqErr *PrereqError
	require.True(t, errors.As(err, &prereqErr))
	// Returns first error
	assert.Equal(t, "go", prereqErr.Binary)
}

func TestCheckPrereqs_UnknownBinary(t *testing.T) {
	lookup := mockLookPath(map[string]bool{})
	err := CheckPrereqs(lookup, "unknown-binary")

	require.Error(t, err)
	var prereqErr *PrereqError
	require.True(t, errors.As(err, &prereqErr))
	assert.Equal(t, "unknown-binary", prereqErr.Binary)
}

func TestCheckPrereqs_NilLookPath_UsesDefault(t *testing.T) {
	// This test just ensures no panic when nil is passed
	// We can't reliably test the actual lookup without system deps
	// So we just verify the nil handling doesn't panic
	_ = CheckPrereqs(nil) // empty list, no checks
}

func TestIsPrereqError_True(t *testing.T) {
	err := &PrereqError{Binary: "go", Message: "test"}
	assert.True(t, IsPrereqError(err))
}

func TestIsPrereqError_False(t *testing.T) {
	err := errors.New("some other error")
	assert.False(t, IsPrereqError(err))
}

func TestPrereqError_ErrorMessage(t *testing.T) {
	err := &PrereqError{Binary: "test", Message: "custom message"}
	assert.Equal(t, "custom message", err.Error())
}
