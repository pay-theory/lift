package security

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStibScanner(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := ioutil.TempDir("", "stib-scanner-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create test files with security issues
	testFiles := map[string]string{
		"hardcoded_credentials.go": `package main

func main() {
	// This has a hardcoded credential
	apiKey := "1234567890abcdef"
	
	// This is fine (using environment variable)
	secretKey := os.Getenv("SECRET_KEY")
}`,

		"weak_crypto.go": `package main

import (
	"crypto/md5"
	"math/rand"
	"time"
)

func generateToken() string {
	// Using weak random number generator
	rand.Seed(time.Now().UnixNano())
	return fmt.Sprintf("%d", rand.Int())
}`,

		"auth_bypass.go": `package main

func authMiddleware(ctx Context) {
	// Skip auth in development
	if env == "development" {
		return next.Handle(ctx)
	}
	
	// Validate token
	token := ctx.Header("Authorization")
}`,

		"debug_leak.go": `package main

func processPayment(payment Payment) {
	if debug {
		log.Printf("Processing payment %+v", payment)
	}
	
	// Process payment
}`,
	}

	// Write test files
	for filename, content := range testFiles {
		filePath := filepath.Join(tmpDir, filename)
		err := ioutil.WriteFile(filePath, []byte(content), 0644)
		require.NoError(t, err)
	}

	// Create scanner and scan the test directory
	scanner := NewStibScanner(tmpDir)
	err = scanner.Scan()
	require.NoError(t, err)

	// Get results
	results := scanner.GetResults()

	// Verify results
	t.Run("DetectsHardcodedCredentials", func(t *testing.T) {
		found := false
		for _, stib := range results {
			if stib.Type == HardcodedCredentials && filepath.Base(stib.FilePath) == "hardcoded_credentials.go" {
				found = true
				break
			}
		}
		assert.True(t, found, "Failed to detect hardcoded credentials")
	})

	t.Run("DetectsWeakCrypto", func(t *testing.T) {
		found := false
		for _, stib := range results {
			if stib.Type == WeakCrypto && filepath.Base(stib.FilePath) == "weak_crypto.go" {
				found = true
				break
			}
		}
		assert.True(t, found, "Failed to detect weak crypto")
	})

	t.Run("DetectsAuthBypass", func(t *testing.T) {
		found := false
		for _, stib := range results {
			if stib.Type == AuthBypass && filepath.Base(stib.FilePath) == "auth_bypass.go" {
				found = true
				break
			}
		}
		assert.True(t, found, "Failed to detect auth bypass")
	})

	t.Run("DetectsDebugLeak", func(t *testing.T) {
		found := false
		for _, stib := range results {
			if stib.Type == DebugLeak && filepath.Base(stib.FilePath) == "debug_leak.go" {
				found = true
				break
			}
		}
		assert.True(t, found, "Failed to detect debug leak")
	})
}

func TestStibScannerWithRealExamples(t *testing.T) {
	// This test scans actual examples from the codebase
	// Skip in CI environments
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping test in CI environment")
	}

	// Get the current directory
	currentDir, err := os.Getwd()
	require.NoError(t, err)

	// Scan the examples directory
	examplesDir := filepath.Join(filepath.Dir(filepath.Dir(currentDir)), "examples")
	scanner := NewStibScanner(examplesDir)
	err = scanner.Scan()
	require.NoError(t, err)

	// Get results
	results := scanner.GetResults()
	
	// Just verify we got some results (the actual number may vary)
	t.Logf("Found %d potential issues in examples directory", len(results))
	
	// Log a few examples of what we found
	for i, stib := range results {
		if i >= 5 {
			break
		}
		t.Logf("Issue %d: [%s] %s in %s:%d", i+1, stib.Severity, stib.Type, stib.FilePath, stib.LineNumber)
	}
}