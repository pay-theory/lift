package liftstate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatePath(t *testing.T) {
	path := StatePath("/project", "dev")
	assert.Equal(t, "/project/.lift/state/dev.json", path)
}

func TestLoad_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	state, err := Load(tmpDir, "dev")
	require.NoError(t, err)
	assert.Nil(t, state)
}

func TestSave_Load_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()

	state := &StageState{
		Version:         1,
		Stage:           "dev",
		BaseDomain:      "example.com",
		StageRootDomain: "dev.example.com",
		Services: map[string]ServiceState{
			"api": {Subdomain: "api", Domain: "api.dev.example.com"},
		},
	}

	err := Save(tmpDir, state)
	require.NoError(t, err)

	// Verify file was created
	statePath := StatePath(tmpDir, "dev")
	require.FileExists(t, statePath)

	// Load it back
	loaded, err := Load(tmpDir, "dev")
	require.NoError(t, err)
	require.NotNil(t, loaded)

	assert.Equal(t, state.Version, loaded.Version)
	assert.Equal(t, state.Stage, loaded.Stage)
	assert.Equal(t, state.BaseDomain, loaded.BaseDomain)
	assert.Equal(t, state.StageRootDomain, loaded.StageRootDomain)
	assert.Equal(t, state.Services, loaded.Services)
}

func TestSave_DefaultsVersion(t *testing.T) {
	tmpDir := t.TempDir()

	state := &StageState{
		Stage:           "dev",
		BaseDomain:      "example.com",
		StageRootDomain: "dev.example.com",
	}

	err := Save(tmpDir, state)
	require.NoError(t, err)

	loaded, err := Load(tmpDir, "dev")
	require.NoError(t, err)
	assert.Equal(t, CurrentVersion, loaded.Version)
}

func TestRemove(t *testing.T) {
	tmpDir := t.TempDir()

	// Create state first
	state := &StageState{
		Stage:           "dev",
		BaseDomain:      "example.com",
		StageRootDomain: "dev.example.com",
	}
	err := Save(tmpDir, state)
	require.NoError(t, err)

	// Verify it exists
	statePath := StatePath(tmpDir, "dev")
	require.FileExists(t, statePath)

	// Remove it
	err = Remove(tmpDir, "dev")
	require.NoError(t, err)

	// Verify it's gone
	_, err = os.Stat(statePath)
	require.True(t, os.IsNotExist(err))
}

func TestRemove_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()

	// Should not error for non-existent file
	err := Remove(tmpDir, "dev")
	require.NoError(t, err)
}

func TestLoad_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Create state directory and invalid file
	stateDir := filepath.Join(tmpDir, StateDir)
	require.NoError(t, os.MkdirAll(stateDir, 0750))
	statePath := filepath.Join(stateDir, "dev.json")
	require.NoError(t, os.WriteFile(statePath, []byte("{ invalid json"), 0600))

	_, err := Load(tmpDir, "dev")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse state file")
}

func TestCheckDomainLock_NilState(t *testing.T) {
	err := CheckDomainLock(nil, "dev", "example.com", "dev.example.com", map[string]string{"api": "api.dev.example.com"})
	require.NoError(t, err)
}

func TestCheckDomainLock_NoMismatch(t *testing.T) {
	current := &StageState{
		Stage:           "dev",
		BaseDomain:      "example.com",
		StageRootDomain: "dev.example.com",
		Services: map[string]ServiceState{
			"api": {Subdomain: "api", Domain: "api.dev.example.com"},
		},
	}

	err := CheckDomainLock(current, "dev", "example.com", "dev.example.com", map[string]string{"api": "api.dev.example.com"})
	require.NoError(t, err)
}

func TestCheckDomainLock_BaseDomainMismatch(t *testing.T) {
	current := &StageState{
		Stage:           "dev",
		BaseDomain:      "old-domain.com",
		StageRootDomain: "dev.old-domain.com",
	}

	err := CheckDomainLock(current, "dev", "new-domain.com", "dev.new-domain.com", nil)
	require.Error(t, err)

	var lockErr *DomainLockError
	require.ErrorAs(t, err, &lockErr)
	assert.Equal(t, "dev", lockErr.Stage)
	assert.Len(t, lockErr.Mismatches, 2) // Both base and stage root changed
}

func TestCheckDomainLock_StageRootMismatch(t *testing.T) {
	current := &StageState{
		Stage:           "dev",
		BaseDomain:      "example.com",
		StageRootDomain: "dev.example.com",
	}

	err := CheckDomainLock(current, "dev", "example.com", "custom-dev.example.com", nil)
	require.Error(t, err)

	var lockErr *DomainLockError
	require.ErrorAs(t, err, &lockErr)
	assert.Len(t, lockErr.Mismatches, 1)
	assert.Equal(t, "stageRootDomain", lockErr.Mismatches[0].Field)
}

func TestCheckDomainLock_ServiceDomainChanged(t *testing.T) {
	current := &StageState{
		Stage:           "dev",
		BaseDomain:      "example.com",
		StageRootDomain: "dev.example.com",
		Services: map[string]ServiceState{
			"api": {Subdomain: "api", Domain: "api.dev.example.com"},
		},
	}

	err := CheckDomainLock(current, "dev", "example.com", "dev.example.com",
		map[string]string{"api": "newapi.dev.example.com"})
	require.Error(t, err)

	var lockErr *DomainLockError
	require.ErrorAs(t, err, &lockErr)
	assert.Len(t, lockErr.Mismatches, 1)
	assert.Contains(t, lockErr.Mismatches[0].Field, "serviceDomain.api")
}

func TestCheckDomainLock_ServiceAdded(t *testing.T) {
	current := &StageState{
		Stage:           "dev",
		BaseDomain:      "example.com",
		StageRootDomain: "dev.example.com",
		Services:        map[string]ServiceState{},
	}

	err := CheckDomainLock(current, "dev", "example.com", "dev.example.com",
		map[string]string{"api": "api.dev.example.com"})
	require.Error(t, err)

	var lockErr *DomainLockError
	require.ErrorAs(t, err, &lockErr)
	assert.Contains(t, lockErr.Error(), "serviceDomain.api")
}

func TestCheckDomainLock_ServiceRemoved(t *testing.T) {
	current := &StageState{
		Stage:           "dev",
		BaseDomain:      "example.com",
		StageRootDomain: "dev.example.com",
		Services: map[string]ServiceState{
			"api": {Subdomain: "api", Domain: "api.dev.example.com"},
		},
	}

	err := CheckDomainLock(current, "dev", "example.com", "dev.example.com", nil)
	require.Error(t, err)

	var lockErr *DomainLockError
	require.ErrorAs(t, err, &lockErr)
	assert.Contains(t, lockErr.Error(), "(removed)")
}

func TestDomainLockError_Message(t *testing.T) {
	err := &DomainLockError{
		Stage: "dev",
		Mismatches: []DomainMismatch{
			{Field: "baseDomain", Current: "old.com", Desired: "new.com"},
		},
	}

	msg := err.Error()
	assert.Contains(t, msg, "domain configuration changed")
	assert.Contains(t, msg, "lift down --stage dev")
	assert.Contains(t, msg, "baseDomain")
	assert.Contains(t, msg, "old.com")
	assert.Contains(t, msg, "new.com")
}
